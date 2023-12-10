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

package conf

import (
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mapstructure instead of json is used for tagging the properties here
// in order to work with spf13/viper unmarshaling

// RESTGatewayConf defines the YAML config structure
type RESTGatewayConf struct {
	MaxInFlight     int             `mapstructure:"maxInFlight"`
	MaxTXWaitTime   int             `mapstructure:"maxTXWaitTime"`
	SendConcurrency int             `mapstructure:"sendConcurrency"`
	Kafka           KafkaConf       `mapstructure:"kafka"`
	Receipts        ReceiptsDBConf  `mapstructure:"receipts"`
	Events          EventstreamConf `mapstructure:"events"`
	HTTP            HTTPConf        `mapstructure:"http"`
	RPC             RPCConf         `mapstructure:"rpc"`
	OpenID          OpenIDConfig    `mapstructure:"openId"`
}

// KafkaConf - Common configuration for Kafka
type KafkaConf struct {
	Brokers       []string `mapstructure:"brokers"`
	ClientID      string   `mapstructure:"clientID"`
	ConsumerGroup string   `mapstructure:"consumerGroup"`
	TopicIn       string   `mapstructure:"topicIn"`
	TopicOut      string   `mapstructure:"topicOut"`
	ProducerFlush struct {
		Frequency int `mapstructure:"frequency"`
		Messages  int `mapstructure:"messages"`
		Bytes     int `mapstructure:"bytes"`
	} `mapstructure:"producerFlush"`
	SASL struct {
		Username string
		Password string
	} `mapstructure:"sasl"`
	TLS TLSConfig `mapstructure:"tls"`
}

type ReceiptsDBConf struct {
	MaxDocs             int                 `mapstructure:"maxDocs"`
	QueryLimit          int                 `mapstructure:"queryLimit"`
	RetryInitialDelayMS int                 `mapstructure:"retryInitialDelay"`
	RetryTimeoutMS      int                 `mapstructure:"retryTimeout"`
	MongoDB             MongoDBReceiptsConf `mapstructure:"mongodb"`
	LevelDB             LevelDBReceiptsConf `mapstructure:"leveldb"`
}

// MongoDBReceiptStoreConf is the configuration for a MongoDB receipt store
type MongoDBReceiptsConf struct {
	URL              string `mapstructure:"url"`
	Database         string `mapstructure:"database"`
	Collection       string `mapstructure:"collection"`
	ConnectTimeoutMS int    `mapstructure:"connectTimeout"`
}

type LevelDBReceiptsConf struct {
	Path string `mapstructure:"path"`
}

type EventstreamConf struct {
	PollingIntervalSec      int                 `mapstructure:"pollingInterval"`
	WebhooksAllowPrivateIPs bool                `json:"webhooksAllowPrivateIPs,omitempty"`
	LevelDB                 LevelDBReceiptsConf `mapstructure:"leveldb"`
}

type RPCConf struct {
	// whether to use the Gateway client in the SDK or
	// relying on the static network described by CCP only
	UseGatewayClient bool `mapstructure:"useGatewayClient"`
	// whether to use the Gateway server with a lightweight SDK
	// only applicable to Fabric node 2.4 or later
	UseGatewayServer bool   `mapstructure:"useGatewayServer"`
	ConfigPath       string `mapstructure:"configPath"`
}

type HTTPConf struct {
	LocalAddr string    `mapstructure:"localAddr"`
	Port      int       `mapstructure:"port"`
	TLS       TLSConfig `mapstructure:"tls"`
}

// TLSConfig is the common TLS config
type TLSConfig struct {
	ClientCertsFile    string `mapstructure:"clientCertsFile"`
	ClientKeyFile      string `mapstructure:"clientKeyFile"`
	CACertsFile        string `mapstructure:"caCertsFile"`
	Enabled            bool   `mapstructure:"enabled"`
	InsecureSkipVerify bool   `mapstructure:"insecureSkipVerify"`
}

// TLSConfig is the common TLS config
type OpenIDConfig struct {
	Host          string `mapstructure:"host"`
	AdminUsername string `mapstructure:"adminUsername"`
	AdminPassword string `mapstructure:"adminPassword"`
	CertFile      string `mapstructure:"certFile"`
	KeyFile       string `mapstructure:"keyFile"`
	AdminRealm    string `mapstructure:"adminRealm"`
	ClientRealm   string `mapstructure:"clientRealm"`
	Group         string `mapstructure:"group"`
}

// CobraInitRPC sets the standard command-line parameters for RPC
func CobraInit(cmd *cobra.Command, conf *RESTGatewayConf) {
	cmd.Flags().IntVarP(&conf.MaxInFlight, "maxinflight", "m", 0, "Maximum messages to hold in-flight")
	_ = viper.BindPFlag("maxinflight", cmd.Flags().Lookup("maxinflight"))
	cmd.Flags().IntVarP(&conf.MaxTXWaitTime, "tx-timeout", "t", 0, "Maximum wait time for an individual transaction (seconds)")
	_ = viper.BindPFlag("maxTXWaitTime", cmd.Flags().Lookup("tx-timeout"))
	cmd.Flags().StringVarP(&conf.HTTP.LocalAddr, "listen-addr", "A", "", "Local address to listen on")
	_ = viper.BindPFlag("http.localAddr", cmd.Flags().Lookup("listen-addr"))
	cmd.Flags().IntVarP(&conf.HTTP.Port, "listen-port", "P", 8080, "Port to listen on")
	_ = viper.BindPFlag("http.port", cmd.Flags().Lookup("listen-port"))

	cmd.Flags().IntVarP(&conf.Receipts.MaxDocs, "receipt-maxdocs", "x", 0, "Receipt store capped size (new collections only)")
	_ = viper.BindPFlag("receipts.maxDocs", cmd.Flags().Lookup("receipt-maxdocs"))
	cmd.Flags().IntVarP(&conf.Receipts.QueryLimit, "receipt-query-limit", "q", 0, "Maximum docs to return on a rest call (cap on limit)")
	_ = viper.BindPFlag("receipts.queryLimit", cmd.Flags().Lookup("receipt-query-limit"))
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.URL, "mongodb-url", "U", "", "MongoDB URL for a receipt store")
	_ = viper.BindPFlag("receipts.mongodb.url", cmd.Flags().Lookup("mongodb-url"))
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Database, "mongodb-database", "D", "", "MongoDB receipt store database")
	_ = viper.BindPFlag("receipts.mongodb.database", cmd.Flags().Lookup("mongodb-database"))
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Collection, "mongodb-receipt-collection", "C", "", "MongoDB receipt store collection")
	_ = viper.BindPFlag("receipts.mongodb.collection", cmd.Flags().Lookup("mongodb-receipt-collection"))
	cmd.Flags().StringVarP(&conf.Receipts.LevelDB.Path, "leveldb-path", "H", "", "Path to LevelDB data directory")
	_ = viper.BindPFlag("receipts.leveldb.path", cmd.Flags().Lookup("leveldb-path"))

	cmd.Flags().StringVarP(&conf.Events.LevelDB.Path, "events-db", "E", "", "Level DB location for subscription management")
	_ = viper.BindPFlag("events.leveldb.path", cmd.Flags().Lookup("events-db"))
	cmd.Flags().IntVarP(&conf.Events.PollingIntervalSec, "events-polling-int", "", 1, "Event polling interval (seconds)")
	_ = viper.BindPFlag("events.pollingInterval", cmd.Flags().Lookup("events-polling-int"))
	cmd.Flags().BoolVarP(&conf.Events.WebhooksAllowPrivateIPs, "events-priv-ips", "", false, "Allow private IPs in Webhooks")
	_ = viper.BindPFlag("events.webhooksAllowPrivateIPs", cmd.Flags().Lookup("events-priv-ips"))

	defBrokerList := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(defBrokerList) == 1 && defBrokerList[0] == "" {
		defBrokerList = []string{}
	}
	defTLSenabled, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_ENABLED"))
	defTLSinsecure, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_INSECURE"))
	cmd.Flags().StringArrayVarP(&conf.Kafka.Brokers, "brokers", "b", defBrokerList, "Comma-separated list of bootstrap brokers")
	_ = viper.BindPFlag("kafka.brokers", cmd.Flags().Lookup("brokers"))
	cmd.Flags().StringVarP(&conf.Kafka.ClientID, "clientid", "i", "", "Client ID (or generated UUID)")
	_ = viper.BindPFlag("kafka.clientID", cmd.Flags().Lookup("clientid"))
	cmd.Flags().StringVarP(&conf.Kafka.ConsumerGroup, "consumer-group", "g", "", "Client ID (or generated UUID)")
	_ = viper.BindPFlag("kafka.consumerGroup", cmd.Flags().Lookup("consumer-group"))
	cmd.Flags().StringVarP(&conf.Kafka.TopicIn, "topic-in", "n", "", "Topic to listen to")
	_ = viper.BindPFlag("kafka.topicIn", cmd.Flags().Lookup("topic-in"))
	cmd.Flags().StringVarP(&conf.Kafka.TopicOut, "topic-out", "o", "", "Topic to send events to")
	_ = viper.BindPFlag("kafka.topicOut", cmd.Flags().Lookup("topic-out"))
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientCertsFile, "tls-clientcerts", "c", "", "Client certificate file, for mutual TLS auth with the Kafka endpoint")
	_ = viper.BindPFlag("kafka.tls.clientCertsFile", cmd.Flags().Lookup("tls-clientcerts"))
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientKeyFile, "tls-clientkey", "k", "", "Client private key file, for mutual TLS auth with the Kafka endpoint")
	_ = viper.BindPFlag("kafka.tls.clientKeyFile", cmd.Flags().Lookup("tls-clientkey"))
	cmd.Flags().StringVarP(&conf.Kafka.TLS.CACertsFile, "tls-cacerts", "a", "", "CA certificates file (or host CAs will be used) when connecting with the Kafka endpoint")
	_ = viper.BindPFlag("kafka.tls.caCertsFile", cmd.Flags().Lookup("tls-cacerts"))
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.Enabled, "tls-enabled", "s", defTLSenabled, "Encrypt network connection with TLS (SSL)")
	_ = viper.BindPFlag("kafka.tls.enabled", cmd.Flags().Lookup("tls-enabled"))
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.InsecureSkipVerify, "tls-insecure", "", defTLSinsecure, "Disable verification of TLS certificate chain")
	_ = viper.BindPFlag("kafka.tls.insecureSkipVerify", cmd.Flags().Lookup("tls-insecure"))
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Username, "sasl-username", "u", "", "Username for SASL authentication")
	_ = viper.BindPFlag("kafka.sasl.username", cmd.Flags().Lookup("sasl-usernam"))
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Password, "sasl-password", "p", "", "Password for SASL authentication")
	_ = viper.BindPFlag("kafka.sasl.password", cmd.Flags().Lookup("sasl-password"))

	cmd.Flags().StringVarP(&conf.RPC.ConfigPath, "rpc-config", "r", "", "Path to the common connection profile YAML for the target Fabric node")
	_ = viper.BindPFlag("rpc.configPath", cmd.Flags().Lookup("rpc-config"))
	cmd.Flags().BoolVarP(&conf.RPC.UseGatewayClient, "gateway-client", "", false, "Whether to use the client-side gateway support when sending transactions")
	_ = viper.BindPFlag("rpc.useGatewayClient", cmd.Flags().Lookup("gateway-client"))
	cmd.Flags().BoolVarP(&conf.RPC.UseGatewayServer, "gateway-server", "", false, "Whether to use the server-side gateway support when sending transactions (Fabric 2.4 or later only)")
	_ = viper.BindPFlag("rpc.useGatewayServer", cmd.Flags().Lookup("gateway-server"))

	cmd.Flags().StringVarP(&conf.OpenID.Host, "openid-host", "", "", "OpenID host url endpoint with port number")
	_ = viper.BindPFlag("openId.host", cmd.Flags().Lookup("openid-host"))
	cmd.Flags().StringVarP(&conf.OpenID.AdminRealm, "openid-admin-realm", "", "", "OpenID realm name")
	_ = viper.BindPFlag("openId.adminRealm", cmd.Flags().Lookup("openid-admin-realm"))
	cmd.Flags().StringVarP(&conf.OpenID.ClientRealm, "openid-client-realm", "", "", "OpenID client name")
	_ = viper.BindPFlag("openId.clientRealm", cmd.Flags().Lookup("openid-client-realm"))
	cmd.Flags().StringVarP(&conf.OpenID.AdminUsername, "openid-admin-username", "", "", "OpenID admin username")
	_ = viper.BindPFlag("openId.adminUsername", cmd.Flags().Lookup("openid-admin-username"))
	cmd.Flags().StringVarP(&conf.OpenID.AdminPassword, "openid-admin-password", "", "", "OpenID realm admin password")
	_ = viper.BindPFlag("openId.adminPassword", cmd.Flags().Lookup("openid-admin-password"))
	cmd.Flags().StringVarP(&conf.OpenID.CertFile, "openid-cert-file", "", "", "OpenID realm admin certificate")
	_ = viper.BindPFlag("openId.certFile", cmd.Flags().Lookup("openid-cert-file"))
	cmd.Flags().StringVarP(&conf.OpenID.KeyFile, "openid-key-file", "", "", "OpenID realm admin private key")
	_ = viper.BindPFlag("openId.keyFile", cmd.Flags().Lookup("openid-key-file"))
	cmd.Flags().StringVarP(&conf.OpenID.Group, "openid-group", "", "", "OpenID realm client group")
	_ = viper.BindPFlag("openId.group", cmd.Flags().Lookup("openid-group"))
}
