package openid

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/Nerzal/gocloak/v13"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
)

// type OpenIdClient interface {
// 	CreateUser(username string, realm string) *string
// 	newOpenIdClient() (openidClientWrapper, error)
// }

type OpenidClientWrapper struct {
	openidClient gocloak.GoCloak
	context      context.Context
	openIdConf   conf.OpenIDConfig
	// identityMgr    msp.IdentityManager
	// caClient       dep.CAClient
	// listeners      []SignerUpdateListener
}

func NewOpenIdClient(o conf.OpenIDConfig) (*OpenidClientWrapper, error) {
	client := gocloak.NewClient(o.Host)
	restyClient := client.RestyClient()
	restyClient.SetDebug(true)
	restyClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	restyClient.SetAllowGetMethodPayload(true)
	ctx := context.Background()

	openidClient := &OpenidClientWrapper{
		openidClient: *client,
		context:      ctx,
		openIdConf:   o,
	}

	return openidClient, nil
}

func (o *OpenidClientWrapper) CreateUser(username string) (*string, error) {

	token, err := o.openidClient.LoginAdmin(o.context, o.openIdConf.AdminUsername, o.openIdConf.AdminPassword, o.openIdConf.AdminRealm)
	if err != nil {
		return nil, err
	}

	log.Println("Query Username")
	log.Println(username)

	usersParams := gocloak.GetUsersParams{
		Username: gocloak.StringP(username),
		// Email:    gocloak.StringP("mail@gmail.com"),
	}

	userId := ""

	users, err := o.openidClient.GetUsers(o.context, token.AccessToken, o.openIdConf.ClientRealm, usersParams)

	log.Println("Query Users:")
	log.Println(len(users))

	if len(users) > 0 {
		for _, user := range users {
			log.Println("COMPARE:")
			log.Println(*user.Username)
			log.Println(username)
			if username == *user.Username {
				log.Println("Existing USER:")
				log.Println(*user.ID)
				log.Println(*user.Username)
				// groups := *user.Groups
				// if len(groups) > 0 {
				// 	fmt.Printf("%v", groups)
				// }
				// if len(*user.RealmRoles) > 0 {
				// 	fmt.Printf("%v", *user.RealmRoles)
				// }

				userId = *user.ID
			}
		}
	}

	if userId != "" {
		return &userId, nil
	} else {

		user := gocloak.User{
			FirstName:     gocloak.StringP(username),
			Email:         gocloak.StringP(username + "@gmail.com"),
			EmailVerified: gocloak.BoolP(true),
			Enabled:       gocloak.BoolP(true),
			Username:      gocloak.StringP(username),
		}

		userId, err = o.openidClient.CreateUser(o.context, token.AccessToken, o.openIdConf.ClientRealm, user)
		if err != nil {
			return nil, err
		}

		o.openidClient.AddUserToGroup(o.context, token.AccessToken, o.openIdConf.ClientRealm, userId, o.openIdConf.Group)
		// o.openidClient.AddClientRoleToUser(o.context, token.AccessToken, o.openIdConf.ClientRealm, userId, o.openIdConf.Group)

		o.openidClient.SetPassword(o.context, token.AccessToken, userId, o.openIdConf.ClientRealm, "123456", false)

		return &userId, nil
	}

}
