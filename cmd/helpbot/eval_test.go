package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseQuestions(t *testing.T) {
	in := `# a comment
first question
spanning two lines
# expect: something
---
USER: hi
BOT: hello
USER: still broken
---
`
	cases, err := parseQuestions(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases", len(cases))
	}
	if c := cases[0]; len(c.turns) != 1 || c.turns[0].text != "first question\nspanning two lines" || c.expect != "something" {
		t.Errorf("case 1 = %+v", c)
	}
	if c := cases[1]; len(c.turns) != 3 || !c.turns[1].assistant || decodeAssistant(c.turns[1].text) != "hello" {
		t.Errorf("case 2 = %+v", c)
	}
}

func TestParseQuestionsRejectsTrailingBot(t *testing.T) {
	if _, err := parseQuestions(strings.NewReader("USER: hi\nBOT: hello\n")); err == nil {
		t.Error("a conversation ending with the bot should be rejected")
	}
}

func TestQuestionsFileParses(t *testing.T) {
	f, err := os.Open("eval/questions.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cases, err := parseQuestions(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 30 {
		t.Errorf("got %d questions, want at least 30", len(cases))
	}
	for i, c := range cases {
		if c.expect == "" {
			t.Errorf("question %d has no expect line", i+1)
		}
	}
}

func TestConfig(t *testing.T) {
	env := map[string]string{
		"HELPBOT_DISCORD_TOKEN": "t", "HELPBOT_GUILD_ID": "g", "HELPBOT_FORUM_CHANNEL_ID": "f",
		"ANTHROPIC_API_KEY": "k", "HELPBOT_STAFF_ROLE_IDS": " 1, 2 ,,3",
	}
	c, err := configFromEnv(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.staffRoleIDs, ",") != "1,2,3" || c.model != "claude-sonnet-5-5" || c.minMemberAge != 10*time.Minute || c.maxReplies != 5 {
		t.Errorf("config = %+v", c)
	}
	delete(env, "HELPBOT_STAFF_ROLE_IDS")
	if _, err := configFromEnv(func(k string) string { return env[k] }); err == nil {
		t.Error("staff roles are required")
	}
	env["HELPBOT_STAFF_ROLE_IDS"], env["HELPBOT_DAILY_BUDGET_USD"] = "1", "lots"
	if _, err := configFromEnv(func(k string) string { return env[k] }); err == nil {
		t.Error("a bad budget should be rejected")
	}
}
