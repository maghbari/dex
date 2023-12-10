// Copyright 2021 Kaleido
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	jwt2 "github.com/hyperledger/firefly-fabconnect/internal/jwt"
	"github.com/hyperledger/firefly-fabconnect/pkg/plugins"
)

type ContextKey int

const (
	ContextKeySystemAuth ContextKey = iota
	ContextKeyAuthContext
	ContextKeyAccessToken
	ContextKeyUsername
	ContextKeySubID
)

var securityModule plugins.SecurityModule

// RegisterSecurityModule is the plug point to register a security module
func RegisterSecurityModule(sm plugins.SecurityModule) {
	securityModule = sm
}

// NewSystemAuthContext creates a system background context
func NewSystemAuthContext() context.Context {
	return context.WithValue(context.Background(), ContextKeySystemAuth, true)
}

// IsSystemContext checks if a context was created as a system context
func IsSystemContext(ctx context.Context) bool {
	b, ok := ctx.Value(ContextKeySystemAuth).(bool)
	return ok && b
}

// WithAuthContext adds an access token to a base context
func WithAuthContext(ctx context.Context, url string, token string, config conf.OpenIDConfig) (context.Context, error) {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}

	httpClient := jwt2.JWKHttpClient{HttpClient: client}

	claimsMap := make(map[string]interface{})

	if url != "/api" && url != "/spec.yaml" && url != "/ws" {

		verifier := jwt2.JwtTokenVerifier{
			// JWKSUri:          "https://iam.mgtappsrv.makeen.ye/realms/makeen/protocol/openid-connect/certs",
			JWKSUri:          fmt.Sprintf("%s/realms/makeen/protocol/openid-connect/certs", config.Host),
			HTTPClient:       &httpClient,
			ClaimsToValidate: claimsMap,
		}

		ctxValue, err := verifier.ValidateToken(ctx, token)
		newToken, _ := verifier.Parse(ctx, token)

		claims := newToken.Claims.(jwt.MapClaims)

		username := claims["preferred_username"]
		sub := claims["sub"]

		// claims = newToken.Claims.(jwt.MapClaims)
		// data := claims["data"].(map[string]interface{})
		// sid := data["sub"].(string)

		fmt.Println(newToken.Valid)
		fmt.Println(username)
		fmt.Println(sub)

		if err != nil {
			return nil, err
		}

		ctx = context.WithValue(ctx, ContextKeyAccessToken, token)
		ctx = context.WithValue(ctx, ContextKeyUsername, username)
		ctx = context.WithValue(ctx, ContextKeySubID, sub)
		ctx = context.WithValue(ctx, ContextKeyAuthContext, ctxValue)

	}

	return ctx, nil

	// if securityModule != nil {
	// 	ctxValue, err := securityModule.VerifyToken(token)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	ctx = context.WithValue(ctx, ContextKeyAccessToken, token)
	// 	ctx = context.WithValue(ctx, ContextKeyAuthContext, ctxValue)
	// 	return ctx, nil
	// }
	// return ctx, nil
}

// GetAuthContext extracts a previously stored auth context from the context
func GetAuthContext(ctx context.Context) interface{} {
	return ctx.Value(ContextKeyAuthContext)
}

// GetAccessToken extracts a previously stored access token
func GetAccessToken(ctx context.Context) string {
	v, ok := ctx.Value(ContextKeyAccessToken).(string)
	if ok {
		return v
	}
	return ""
}

// AuthRPC authorize an RPC call
func AuthRPC(ctx context.Context, method string, args ...interface{}) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthRPC(authCtx, method, args...)
	}
	return nil
}

// AuthRPCSubscribe authorize a subscribe RPC call
func AuthRPCSubscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthRPCSubscribe(authCtx, namespace, channel, args...)
	}
	return nil
}

// AuthEventStreams authorize the whole of event streams
func AuthEventStreams(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthEventStreams(authCtx)
	}
	return nil
}

// AuthListAsyncReplies authorize the listing or searching of all replies
func AuthListAsyncReplies(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthListAsyncReplies(authCtx)
	}
	return nil
}

// AuthReadAsyncReplyByUUID authorize the query of an invidual reply by UUID
func AuthReadAsyncReplyByUUID(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthReadAsyncReplyByUUID(authCtx)
	}
	return nil
}
