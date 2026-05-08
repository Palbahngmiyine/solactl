package main

import "testing"

func TestInvokesCRMDetectsCommandAfterRootFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "plain crm", args: []string{"crm", "records", "list"}, want: true},
		{name: "profile before crm", args: []string{"--profile", "prod", "crm", "records", "list"}, want: true},
		{name: "profile equals form before crm", args: []string{"--profile=prod", "crm", "records", "list"}, want: true},
		{name: "api credentials before crm", args: []string{"--api-key", "key", "--api-secret", "secret", "crm", "records", "list"}, want: true},
		{name: "bool flags before crm", args: []string{"--debug", "--json", "crm", "records", "list"}, want: true},
		{name: "flag value named crm is not command", args: []string{"--profile", "crm", "balance"}, want: false},
		{name: "non crm command after flag", args: []string{"--timeout", "5s", "send", "sms"}, want: false},
		{name: "double dash crm", args: []string{"--", "crm", "records"}, want: true},
		{name: "empty args", args: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := invokesCRM(tt.args); got != tt.want {
				t.Fatalf("invokesCRM(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
