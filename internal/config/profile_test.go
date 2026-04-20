package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempRoot points RootDir at a temp dir for the duration of the test
// by setting XDG_CONFIG_HOME, and resets the active profile to default.
func withTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	prev := activeProfile
	activeProfile = ""
	t.Cleanup(func() { activeProfile = prev })
	return filepath.Join(dir, "fylla")
}

func TestValidateProfileName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"work", false},
		{"Work", false},
		{"personal_1", false},
		{"a1_B2", false},
		{"", true},
		{".hidden", true},
		{"has-dash", true},
		{"has space", true},
		{"has/slash", true},
		{"current", true},
		{"profiles", true},
		{"config", true},
	}
	for _, c := range cases {
		err := ValidateProfileName(c.name)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateProfileName(%q) err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestCreateAndListProfiles(t *testing.T) {
	withTempRoot(t)

	if err := CreateProfile("work", ""); err != nil {
		t.Fatalf("CreateProfile(work): %v", err)
	}
	if err := CreateProfile("home", ""); err != nil {
		t.Fatalf("CreateProfile(home): %v", err)
	}
	if err := CreateProfile("work", ""); err == nil {
		t.Fatal("expected error creating existing profile")
	}

	names, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(names) != 2 || names[0] != "home" || names[1] != "work" {
		t.Errorf("ListProfiles = %v, want [home work]", names)
	}

	workCfg := filepath.Join(filepath.Dir(filepath.Dir(must(ProfileDirFor("work")))), "profiles", "work", "config.yaml")
	if _, err := os.Stat(workCfg); err != nil {
		t.Errorf("work config not seeded: %v", err)
	}
}

func TestCreateProfileFromCopiesFiles(t *testing.T) {
	withTempRoot(t)

	if err := CreateProfile("src", ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	srcDir := must(ProfileDirFor("src"))
	if err := os.WriteFile(filepath.Join(srcDir, "kendo_credentials.json"), []byte(`{"token":"t"}`), 0600); err != nil {
		t.Fatalf("seed credential: %v", err)
	}

	if err := CreateProfile("copy", "src"); err != nil {
		t.Fatalf("CreateProfile copy: %v", err)
	}
	copyDir := must(ProfileDirFor("copy"))
	data, err := os.ReadFile(filepath.Join(copyDir, "kendo_credentials.json"))
	if err != nil {
		t.Fatalf("copied credential missing: %v", err)
	}
	if string(data) != `{"token":"t"}` {
		t.Errorf("copied credential content = %q", string(data))
	}
}

func TestDeleteProfile(t *testing.T) {
	withTempRoot(t)
	if err := CreateProfile("victim", ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := CreateProfile("keeper", ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := WritePointer("keeper"); err != nil {
		t.Fatalf("WritePointer: %v", err)
	}

	if err := DeleteProfile("keeper", false); err == nil {
		t.Fatal("expected refusal to delete current profile")
	}
	if err := DeleteProfile("victim", false); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	exists, _ := ProfileExists("victim")
	if exists {
		t.Error("victim still exists")
	}
	if err := DeleteProfile("keeper", true); err != nil {
		t.Fatalf("DeleteProfile force: %v", err)
	}
}

func TestResolveProfilePrecedence(t *testing.T) {
	withTempRoot(t)
	if err := CreateProfile(DefaultProfileName, ""); err != nil {
		t.Fatalf("CreateProfile default: %v", err)
	}
	if err := CreateProfile("work", ""); err != nil {
		t.Fatalf("CreateProfile work: %v", err)
	}
	if err := CreateProfile("env_one", ""); err != nil {
		t.Fatalf("CreateProfile env_one: %v", err)
	}
	if err := WritePointer("work"); err != nil {
		t.Fatalf("WritePointer: %v", err)
	}

	// Flag wins.
	t.Setenv("FYLLA_PROFILE", "env_one")
	name, err := ResolveProfile("env_one")
	if err != nil {
		t.Fatalf("ResolveProfile flag: %v", err)
	}
	if name != "env_one" {
		t.Errorf("flag: got %q", name)
	}

	// Env beats pointer.
	name, err = ResolveProfile("")
	if err != nil {
		t.Fatalf("ResolveProfile env: %v", err)
	}
	if name != "env_one" {
		t.Errorf("env: got %q", name)
	}

	// Pointer when no env.
	t.Setenv("FYLLA_PROFILE", "")
	name, err = ResolveProfile("")
	if err != nil {
		t.Fatalf("ResolveProfile pointer: %v", err)
	}
	if name != "work" {
		t.Errorf("pointer: got %q", name)
	}
}

func TestResolveProfileBadFlagErrors(t *testing.T) {
	withTempRoot(t)
	if err := CreateProfile(DefaultProfileName, ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if _, err := ResolveProfile("nope"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestResolveProfileStalePointerFallsBackToDefault(t *testing.T) {
	withTempRoot(t)
	if err := CreateProfile(DefaultProfileName, ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := WritePointer("gone"); err != nil {
		t.Fatalf("WritePointer: %v", err)
	}
	name, err := ResolveProfile("")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if name != DefaultProfileName {
		t.Errorf("got %q, want %q", name, DefaultProfileName)
	}
}

func TestMigrateLegacyLayout(t *testing.T) {
	root := withTempRoot(t)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	legacy := map[string][]byte{
		"config.yaml":                []byte("providers: [kendo]\n"),
		"timer.json":                 []byte("{}\n"),
		"kendo_credentials.json":     []byte(`{"token":"k"}`),
		"google_credentials.json":    []byte(`{}`),
	}
	for name, data := range legacy {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	migrated, err := MigrateLegacyLayout()
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to happen")
	}

	dest := filepath.Join(root, "profiles", DefaultProfileName)
	for name := range legacy {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("%s not moved: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present in root", name)
		}
	}
	ptr, err := ReadPointer()
	if err != nil {
		t.Fatalf("ReadPointer: %v", err)
	}
	if ptr != DefaultProfileName {
		t.Errorf("pointer = %q, want %q", ptr, DefaultProfileName)
	}
}

func TestMigrateLegacyLayoutNoop(t *testing.T) {
	withTempRoot(t)
	// profiles/ already exists → no migration.
	if err := CreateProfile(DefaultProfileName, ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	migrated, err := MigrateLegacyLayout()
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if migrated {
		t.Error("expected no migration when profiles/ exists")
	}
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
