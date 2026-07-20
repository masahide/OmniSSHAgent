//go:build windows

package autostart

import "testing"

func TestQuoteExecutable(t *testing.T) {
	const executable = `C:\Program Files\OmniSSHAgent\OmniSSHAgent.exe`
	if got, want := quoteExecutable(executable), `"C:\Program Files\OmniSSHAgent\OmniSSHAgent.exe"`; got != want {
		t.Fatalf("quoteExecutable()=%q, want %q", got, want)
	}
}

func TestCommandTargetsExecutable(t *testing.T) {
	const executable = `C:\Users\Test\AppData\Local\Programs\OmniSSHAgent\OmniSSHAgent.exe`
	for _, test := range []struct {
		name    string
		command string
		want    bool
	}{
		{"quoted", `"` + executable + `"`, true},
		{"unquoted", executable, true},
		{"case insensitive", `"c:\users\test\appdata\local\programs\omnisshagent\omnisshagent.exe"`, true},
		{"surrounding whitespace", `  "` + executable + `"  `, true},
		{"different executable", `"C:\Other\OmniSSHAgent.exe"`, false},
		{"unexpected arguments", `"` + executable + `" --flag`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := commandTargetsExecutable(test.command, executable); got != test.want {
				t.Fatalf("commandTargetsExecutable(%q)=%v, want %v", test.command, got, test.want)
			}
		})
	}
}
