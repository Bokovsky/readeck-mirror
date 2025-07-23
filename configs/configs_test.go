// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package configs

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzGenerateKey(f *testing.F) {
	for i := range 50 {
		f.Add(i)
	}

	f.Fuzz(func(t *testing.T, _ int) {
		k := GenerateKey()
		if len(k) != 64 {
			t.Fatalf("wrong length, got: %d, wanted %d", len(k), 64)
		}
	})
}

func TestEnvVars(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		expect func(*require.Assertions, config, error)
	}{
		{"READECK_LOG_LEVEL", "warn", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(slog.LevelWarn, cf.Main.LogLevel)
		}},
		{"READECK_LOG_LEVEL", "debug-10", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(slog.LevelDebug-10, cf.Main.LogLevel)
		}},
		{"READECK_LOG_FORMAT", "json", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("json", cf.Main.LogFormat)
		}},
		{"READECK_SECRET_KEY", "abcdefghijkl", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("abcdefghijkl", cf.Main.SecretKey)

			v, exists := os.LookupEnv("READECK_SECRET_KEY")
			assert.Empty(v)
			assert.False(exists)
		}},
		{"READECK_DATA_DIRECTORY", "/srv/data/readeck", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("/srv/data/readeck", cf.Main.DataDirectory)

			v, exists := os.LookupEnv("READECK_DATA_DIRECTORY")
			assert.Empty(v)
			assert.False(exists)
		}},
		{"READECK_SERVER_BASE_URL", "http://example.net/", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("http://example.net/", cf.Server.BaseURL.String())
		}},
		{"READECK_SERVER_BASE_URL", "https://example.net/.//app///", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			cf.Server.BaseURL.normalize()
			assert.Equal("https://example.net/app/", cf.Server.BaseURL.String())
		}},
		{"READECK_SERVER_HOST", "localhost", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("localhost", cf.Server.Host)
		}},
		{"READECK_SERVER_PORT", "8000", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(8000, cf.Server.Port)
		}},
		{"READECK_SERVER_PREFIX", "/app", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("/app", cf.Server.Prefix)
		}},
		{"READECK_TRUSTED_PROXIES", "127.0.0.2,192.168.0.1/26,fd00:abcd::/64", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			r := []string{}
			for _, x := range cf.Server.TrustedProxies {
				r = append(r, x.String())
			}
			assert.Equal([]string{"127.0.0.2/32", "192.168.0.0/26", "fd00:abcd::/64"}, r)

			v, exists := os.LookupEnv("READECK_TRUSTED_PROXIES")
			assert.Empty(v)
			assert.False(exists)
		}},
		{"READECK_ALLOWED_HOSTS", "example.net,example.com", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal([]string{"example.net", "example.com"}, cf.Server.AllowedHosts)
		}},
		{"READECK_DATABASE_SOURCE", "sqlite3::memory", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("sqlite3::memory", cf.Database.Source)

			v, exists := os.LookupEnv("READECK_DATABASE_SOURCE")
			assert.Empty(v)
			assert.False(exists)
		}},
		{"READECK_MAIL_FROM", "", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Nil(cf.Email.From.Address)
		}},
		{"READECK_MAIL_FROM", "test@example.net", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("test@example.net", cf.Email.From.Addr())
			assert.Equal("<test@example.net>", cf.Email.From.String())
		}},
		{"READECK_MAIL_FROM", "Readeck <test@example.net>", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("test@example.net", cf.Email.From.Addr())
			assert.Equal(`"Readeck" <test@example.net>`, cf.Email.From.String())
		}},
		{"READECK_MAIL_FROM", "test@example.net (Readeck)", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("test@example.net", cf.Email.From.Addr())
			assert.Equal(`"Readeck" <test@example.net>`, cf.Email.From.String())
		}},
		{"READECK_PUBLIC_SHARE_TTL", "48", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(48, cf.Bookmarks.PublicShareTTL)
		}},
		{"READECK_WORKER_DSN", "memory://", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("memory://", cf.Worker.DSN)

			v, exists := os.LookupEnv("READECK_WORKER_DSN")
			assert.Empty(v)
			assert.False(exists)
		}},
		{"READECK_WORKER_NUMBER", "10", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(10, cf.Worker.NumWorkers)
		}},
		{"READECK_WORKER_START", "true", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.True(cf.Worker.StartWorker)
		}},
		{"READECK_METRICS_HOST", "::1", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal("::1", cf.Metrics.Host)
		}},
		{"READECK_METRICS_PORT", "8002", func(assert *require.Assertions, cf config, err error) {
			assert.NoError(err)
			assert.Equal(8002, cf.Metrics.Port)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cf := config{}
			t.Setenv(test.name, test.value)
			err := cf.LoadEnv()
			test.expect(require.New(t), cf, err)
		})
	}

	t.Run("email settings", func(t *testing.T) {
		envMap := map[string]string{
			"READECK_MAIL_DEBUG":       "false",
			"READECK_MAIL_HOST":        "localhost",
			"READECK_MAIL_PORT":        "25",
			"READECK_MAIL_USERNAME":    "alice",
			"READECK_MAIL_PASSWORD":    "1234",
			"READECK_MAIL_ENCRYPTION":  "starttls",
			"READECK_MAIL_INSECURE":    "true",
			"READECK_MAIL_FROM":        "alice@example.net",
			"READECK_MAIL_FROMNOREPLY": "Readeck <noreply@example.net>",
		}

		for k, v := range envMap {
			t.Setenv(k, v)
		}
		cf := config{}
		err := cf.LoadEnv()

		assert := require.New(t)
		assert.NoError(err)
		assert.False(cf.Email.Debug)
		assert.Equal("localhost", cf.Email.Host)
		assert.Equal(25, cf.Email.Port)
		assert.Equal("alice", cf.Email.Username)
		assert.Equal("1234", cf.Email.Password)
		assert.Equal("starttls", cf.Email.Encryption)
		assert.True(cf.Email.Insecure)
		assert.Equal("alice@example.net", cf.Email.From.Addr())
		assert.Equal("noreply@example.net", cf.Email.FromNoReply.Addr())

		for k := range envMap {
			v, exists := os.LookupEnv(k)
			assert.Empty(v)
			assert.False(exists)
		}
	})
}
