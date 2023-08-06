package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var defaultServerConfig = newServerConfig()

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    serverConfig
		wantErr     bool
		expectedErr error
	}{
		{
			name:        "empty config results in error",
			input:       "",
			expected:    defaultServerConfig,
			wantErr:     true,
			expectedErr: errNoDomainsConfigured,
		},
		{
			name: "single domain config",
			input: `
port = 1966
readTimeout = "15s"
writeTimeout = "1m"

[domain.localhost]
path = "localhost/gemini"
certFilePath = "certs/localhost.cert"
keyFilePath = "certs/localhost.key"

[domain.domainb]
path = "domainb/gemini"
certFilePath = "certs/domainb.cert"
keyFilePath = "certs/domainb.key"
			`,
			expected: serverConfig{Port: 1966,
				ReadTimeout:  time.Second * 15,
				WriteTimeout: time.Minute,
				Domain: map[string]domainConfig{
					"localhost": {
						Path:         "localhost/gemini",
						CertFilePath: "certs/localhost.cert",
						KeyFilePath:  "certs/localhost.key",
					},
					"domainb": {
						Path:         "domainb/gemini",
						CertFilePath: "certs/domainb.cert",
						KeyFilePath:  "certs/domainb.key",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := loadConfigFile(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}
