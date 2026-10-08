package main

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const box = "---\n####\n#  #\n####\n"

func writeSigil(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+sigilExt), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLayersOverrideByFilename(t *testing.T) {
	root := t.TempDir()
	builtin, user, project := filepath.Join(root, "b"), filepath.Join(root, "u"), filepath.Join(root, "p")
	writeSigil(t, builtin, "alpha", "skill: alpha\n"+box)
	writeSigil(t, builtin, "beta", "skill: beta\n"+box)
	writeSigil(t, user, "alpha", "skill: alpha-mine\n"+box)
	writeSigil(t, user, "gamma", "prompt: do the gamma thing\n"+box)
	writeSigil(t, project, "beta", "disabled: true\n---\n")
	writeSigil(t, project, "broken", "skill: x\n")

	spells, problems := LoadSpellDirs([]SpellDir{{builtin, "built-in"}, {user, "user"}, {project, "project"}})
	var got []string
	for _, s := range spells {
		got = append(got, s.ID+"="+s.Skill+s.Prompt+"@"+s.Source)
	}
	want := "alpha=alpha-mine@user gamma=do the gamma thing@user"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "broken.sigil") {
		t.Errorf("problems = %v, want one for broken.sigil", problems)
	}
}

func TestInscribeWritesUserSigilFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	builtin := t.TempDir()
	writeSigil(t, builtin, "code-review", "name: Eye of Scrutiny\nskill: code-review\n"+box)
	b, err := LoadSpellbook("", []SpellDir{{builtin, "built-in"}, {userSpellsDir(), "user"}})
	if err != nil {
		t.Fatal(err)
	}
	stroke := []Pt{{0, 0, 0}, {10, 0, 0}, {10, 10, 0}, {0, 10, 0}}

	// Overriding a built-in keeps its header and drawings, then adds the new one.
	path, n, err := b.Inscribe("code-review", stroke)
	if err != nil || n != 2 {
		t.Fatalf("inscribe code-review: n=%d err=%v", n, err)
	}
	data, _ := os.ReadFile(path)
	s, err := ParseSigil("code-review", string(data))
	if err != nil || s.Name != "Eye of Scrutiny" || len(s.Drawings) != 2 {
		t.Fatalf("user file %s: %v %+v", path, err, s)
	}

	// A new id becomes a spell for the skill of the same name.
	if _, n, err = b.Inscribe("deploy-staging", stroke); err != nil || n != 1 {
		t.Fatalf("inscribe new: n=%d err=%v", n, err)
	}
	b, _ = LoadSpellbook("", []SpellDir{{builtin, "built-in"}, {userSpellsDir(), "user"}})
	if i := b.indexOf("deploy-staging"); i < 0 || b.Spells[i].Skill != "deploy-staging" || b.Spells[i].Name != "Deploy Staging" {
		t.Fatalf("new spell not loaded: %+v", b.Spells)
	}
}

func TestMigrateLegacySpellbook(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings := userSpellbookPath()
	os.MkdirAll(filepath.Dir(settings), 0o755)
	legacy := `{"glow": "off", "spells": [
		{"name": "Circle of Purity", "shape": "circle", "skill": "simplify"},
		{"name": "My Deploy", "skill": "deploy", "args": "prod", "templates": [[[0,0,0],[10,0,0],[10,10,0],[0,10,0],[0,0,0]]]}
	]}`
	os.WriteFile(settings, []byte(legacy), 0o644)

	b, err := LoadSpellbook("", []SpellDir{{userSpellsDir(), "user"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Spells) != 1 || b.Spells[0].ID != "my-deploy" {
		t.Fatalf("legacy load: want only the trained spell, got %+v", b.Spells)
	}
	written, err := b.Migrate(settings)
	if err != nil || len(written) != 1 {
		t.Fatalf("migrate: %v %v", written, err)
	}
	data, _ := os.ReadFile(written[0])
	if s, err := ParseSigil("my-deploy", string(data)); err != nil || s.Skill != "deploy" || s.Args != "prod" {
		t.Fatalf("migrated file: %v\n%s", err, data)
	}
	var raw map[string]any
	data, _ = os.ReadFile(settings)
	json.Unmarshal(data, &raw)
	if _, ok := raw["spells"]; ok || raw["glow"] != "off" || len(raw) != 1 {
		t.Fatalf("settings after migrate: %s", data)
	}
	b, _ = LoadSpellbook("", []SpellDir{{userSpellsDir(), "user"}})
	if len(b.Spells) != 1 || b.Spells[0].Source != "user" {
		t.Fatalf("after migrate: %+v", b.Spells)
	}
}

func TestAttemptLogAndReplay(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	spells, _ := LoadSpellDirs([]SpellDir{{"../spells", "built-in"}})
	b := &Spellbook{MaxDistance: defaultMaxDistance, Spells: spells}
	pts := wobble(shapes["circle"](), rand.New(rand.NewPCG(1, 1)))
	logAttempt(b, pts, b.Recognizer().Recognize(pts))
	data, err := os.ReadFile(attemptLogPath())
	if err != nil {
		t.Fatal(err)
	}
	var a Attempt
	if err := json.Unmarshal(data, &a); err != nil || a.Best != "simplify" || !a.Cast || len(a.Pts) != len(pts) {
		t.Fatalf("logged %s (%v)", data, err)
	}
	Replay(b) // must re-read and re-score without panicking
}
