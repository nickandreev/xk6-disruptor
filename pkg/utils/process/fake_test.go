package process

import (
	"fmt"
	"strings"
	"testing"
)

func Test_FakeExecutor(t *testing.T) {
	testCases := []struct {
		title   string
		cmdLine string
		out     []byte
		err     error
	}{
		{
			title:   "command with arguments",
			cmdLine: "cat -n hello world",
			out:     []byte("hello world"),
			err:     nil,
		},
		{
			title:   "command without arguments",
			cmdLine: "/bin/true",
			out:     []byte{},
			err:     nil,
		},
		{
			title:   "command returning error",
			cmdLine: "/bin/false",
			out:     []byte{},
			err:     fmt.Errorf("command exited with rc 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			fake := NewFakeExecutor(tc.out, tc.err)
			cmd := strings.Split(tc.cmdLine, " ")[0]
			args := strings.Split(tc.cmdLine, " ")[0:]
			out, err := fake.Exec(cmd, args...)

			if !fake.Invoked() {
				t.Error("Invoked method should return true")
				return
			}

			if err != tc.err {
				t.Errorf(
					"returned output does not match expected value.\n"+
						"Expected: %v\nActual: %v\n",
					tc.err,
					err,
				)
				return
			}

			if string(out) != string(tc.out) {
				t.Errorf(
					"returned output does not match expected value.\n"+
						"Expected: %s\nActual: %s\n",
					string(tc.out),
					string(out),
				)
				return
			}
		})
	}
}

func Test_MultipleExecutions(t *testing.T) {
	testCases := []struct {
		title    string
		cmdLines []string
		out      []byte
		err      error
	}{
		{
			title:    "command with arguments",
			cmdLines: []string{"cat -n 'hello'", "cat -n 'world'"},
			out:      []byte{},
			err:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			fake := NewFakeExecutor(tc.out, tc.err)
			// execute a sequence of commands
			for _, cmdline := range tc.cmdLines {
				cmd := strings.Split(cmdline, " ")[0]
				args := strings.Split(cmdline, " ")[1:]
				out, err := fake.Exec(cmd, args...)

				if err != tc.err {
					t.Errorf(
						"returned output does not match expected value.\n"+
							"Expected: %v\nActual: %v\n",
						tc.err,
						err,
					)
					return
				}

				if string(out) != string(tc.out) {
					t.Errorf(
						"returned output does not match expected value.\n"+
							"Expected: %s\nActual: %s\n",
						string(tc.out),
						string(out),
					)
					return
				}
			}
			// check history of commands is the same sequence than executed
			expected := strings.Join(tc.cmdLines, "\n")
			actual := strings.Join(fake.CmdHistory(), "\n")
			if actual != expected {
				t.Errorf(
					"command history does not match expected value.\n"+
						"Expected: %v\nActual: %v\n",
					expected,
					actual,
				)
			}
		})
	}
}

func Test_Callbacks(t *testing.T) {
	testCases := []struct {
		title   string
		cmdLine string
		out     []byte
		err     error
	}{
		{
			title:   "command with arguments",
			cmdLine: "cat -n hello world",
			out:     []byte("hello world"),
			err:     nil,
		},
		{
			title:   "command without arguments",
			cmdLine: "/bin/true",
			out:     []byte{},
			err:     nil,
		},
		{
			title:   "command returning error",
			cmdLine: "/bin/false",
			out:     []byte{},
			err:     fmt.Errorf("command exited with rc 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			fake := NewCallbackExecutor(func(cmd string, args ...string) ([]byte, error) {
				return tc.out, tc.err
			})

			cmd := strings.Split(tc.cmdLine, " ")[0]
			args := strings.Split(tc.cmdLine, " ")[0:]
			out, err := fake.Exec(cmd, args...)

			if !fake.Invoked() {
				t.Error("Invoked method should return true")
				return
			}

			if err != tc.err {
				t.Errorf(
					"returned output does not match expected value.\n"+
						"Expected: %v\nActual: %v\n",
					tc.err,
					err,
				)
				return
			}

			if string(out) != string(tc.out) {
				t.Errorf(
					"returned output does not match expected value.\n"+
						"Expected: %s\nActual: %s\n",
					string(tc.out),
					string(out),
				)
				return
			}
		})
	}
}
