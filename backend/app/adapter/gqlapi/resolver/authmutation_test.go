// +build !integration all

package resolver

import (
	"testing"
	"time"

	"github.com/short-d/app/fw/assert"
	"github.com/short-d/app/fw/crypto"
	"github.com/short-d/app/fw/timer"
	"github.com/short-d/short/backend/app/entity"
	"github.com/short-d/short/backend/app/usecase/authenticator"
	"github.com/short-d/short/backend/app/usecase/authorizer"
	"github.com/short-d/short/backend/app/usecase/authorizer/rbac"
	"github.com/short-d/short/backend/app/usecase/authorizer/rbac/role"
	"github.com/short-d/short/backend/app/usecase/changelog"
	"github.com/short-d/short/backend/app/usecase/keygen"
	"github.com/short-d/short/backend/app/usecase/repository"
	"github.com/short-d/short/backend/app/usecase/risk"
	"github.com/short-d/short/backend/app/usecase/shortlink"
	"github.com/short-d/short/backend/app/usecase/validator"
)

func TestUpdateShortLink(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	newAlias := "NewAlias"
	newLongLink := "https://www.short-d.com"
	maliciousURL := "http://malware.wicar.org/data/ms14_064_ole_not_xp.html"
	shortLinks := shortLinkMap{
		"SimpleAlias": entity.ShortLink{
			Alias:    "SimpleAlias",
			LongLink: "https://www.google.com/",
		},
	}
	testCases := []struct {
		name               string
		args               *UpdateShortLinkArgs
		user               entity.User
		shortLinks         shortLinkMap
		relationUsers      []entity.User
		relationShortLinks []entity.ShortLink
		expectedShortLink  *ShortLink
		hasError           bool
	}{
		{
			name: "empty update returns empty response",
			args: &UpdateShortLinkArgs{
				OldAlias:  "SimpleAlias",
				ShortLink: ShortLinkInput{},
			},
			user: entity.User{
				ID:    "1",
				Email: "short@gmail.com",
			},
			shortLinks: shortLinks,
			relationUsers: []entity.User{
				{
					ID:    "1",
					Email: "short@gmail.com",
				},
			},
			relationShortLinks: []entity.ShortLink{
				{
					Alias:    "SimpleAlias",
					LongLink: "https://www.google.com/",
				},
			},
			expectedShortLink: nil,
			hasError:          false,
		},
		{
			name: "update only alias",
			args: &UpdateShortLinkArgs{
				OldAlias: "SimpleAlias",
				ShortLink: ShortLinkInput{
					CustomAlias: &newAlias,
				},
			},
			user: entity.User{
				ID:    "1",
				Email: "short@gmail.com",
			},
			shortLinks: shortLinks,
			relationUsers: []entity.User{
				{
					ID:    "1",
					Email: "short@gmail.com",
				},
			},
			relationShortLinks: []entity.ShortLink{
				{
					Alias:    "SimpleAlias",
					LongLink: "https://www.google.com/",
				},
			},
			expectedShortLink: &ShortLink{
				shortLink: entity.ShortLink{
					Alias:    newAlias,
					LongLink: "https://www.google.com/",
				},
			},
			hasError: false,
		},
		{
			name: "update only long link",
			args: &UpdateShortLinkArgs{
				OldAlias: "SimpleAlias",
				ShortLink: ShortLinkInput{
					LongLink: &newLongLink,
				},
			},
			user: entity.User{
				ID:    "1",
				Email: "short@gmail.com",
			},
			shortLinks: shortLinks,
			relationUsers: []entity.User{
				{
					ID:    "1",
					Email: "short@gmail.com",
				},
			},
			relationShortLinks: []entity.ShortLink{
				{
					Alias:    "SimpleAlias",
					LongLink: "https://www.google.com/",
				},
			},
			expectedShortLink: &ShortLink{
				shortLink: entity.ShortLink{
					Alias:    "SimpleAlias",
					LongLink: newLongLink,
				},
			},
			hasError: false,
		},
		{
			name: "update both alias and long link",
			args: &UpdateShortLinkArgs{
				OldAlias: "SimpleAlias",
				ShortLink: ShortLinkInput{
					CustomAlias: &newAlias,
					LongLink:    &newLongLink,
				},
			},
			user: entity.User{
				ID:    "1",
				Email: "short@gmail.com",
			},
			shortLinks: shortLinks,
			relationUsers: []entity.User{
				{
					ID:    "1",
					Email: "short@gmail.com",
				},
			},
			relationShortLinks: []entity.ShortLink{
				{
					Alias:    "SimpleAlias",
					LongLink: "https://www.google.com/",
				},
			},
			expectedShortLink: &ShortLink{
				shortLink: entity.ShortLink{
					Alias:    newAlias,
					LongLink: newLongLink,
				},
			},
			hasError: false,
		},
		{
			name: "update long link to malicious url",
			args: &UpdateShortLinkArgs{
				OldAlias: "SimpleAlias",
				ShortLink: ShortLinkInput{
					LongLink: &maliciousURL,
				},
			},
			user: entity.User{
				ID:    "1",
				Email: "short@gmail.com",
			},
			shortLinks: shortLinks,
			relationUsers: []entity.User{
				{
					ID:    "1",
					Email: "short@gmail.com",
				},
			},
			relationShortLinks: []entity.ShortLink{
				{
					Alias:    "SimpleAlias",
					LongLink: "https://www.google.com/",
				},
			},
			hasError: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			blockedHash := map[string]bool{
				"http://malware.wicar.org/data/ms14_064_ole_not_xp.html": false,
			}
			blacklist := risk.NewBlackListFake(blockedHash)
			shortLinkRepo := repository.NewShortLinkFake(testCase.shortLinks)
			userShortLinkRepo := repository.NewUserShortLinkRepoFake(
				testCase.relationUsers,
				testCase.relationShortLinks,
			)

			keyFetcher := keygen.NewKeyFetcherFake([]keygen.Key{})
			keyGen, err := keygen.NewKeyGenerator(2, &keyFetcher)
			assert.Equal(t, nil, err)

			longLinkValidator := validator.NewLongLink()
			aliasValidator := validator.NewCustomAlias()
			riskDetector := risk.NewDetector(blacklist)

			tm := timer.NewStub(now)
			changeLogRepo := repository.NewChangeLogFake([]entity.Change{})
			userChangeLogRepo := repository.NewUserChangeLogFake(map[string]time.Time{})

			fakeRolesRepo := repository.NewUserRoleFake(map[string][]role.Role{})
			rb := rbac.NewRBAC(fakeRolesRepo)
			au := authorizer.NewAuthorizer(rb)

			changeLog := changelog.NewPersist(keyGen, tm, &changeLogRepo, &userChangeLogRepo, au)

			tokenizer := crypto.NewTokenizerFake()
			auth := authenticator.NewAuthenticator(tokenizer, tm, time.Hour)

			authToken, err := auth.GenerateToken(testCase.user)
			assert.Equal(t, nil, err)

			creator := shortlink.NewCreatorPersist(
				&shortLinkRepo,
				&userShortLinkRepo,
				keyGen,
				longLinkValidator,
				aliasValidator,
				tm,
				riskDetector,
			)
			updater := shortlink.NewUpdaterPersist(
				&shortLinkRepo,
				&userShortLinkRepo,
				longLinkValidator,
				aliasValidator,
				tm,
				riskDetector,
			)
			authMutation := newAuthMutation(
				&authToken,
				auth,
				changeLog,
				creator,
				updater,
			)
			shortLink, err := authMutation.UpdateShortLink(testCase.args)
			if testCase.hasError {
				assert.NotEqual(t, nil, err)
				return
			}
			assert.Equal(t, nil, err)
			if shortLink == nil {
				return
			}
			assert.Equal(t, testCase.expectedShortLink.shortLink.Alias, shortLink.shortLink.Alias)
			assert.Equal(t, testCase.expectedShortLink.shortLink.LongLink, shortLink.shortLink.LongLink)
			assert.Equal(t, true, shortLink.shortLink.UpdatedAt.After(now))
		})
	}
}