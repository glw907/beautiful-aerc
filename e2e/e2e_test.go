package e2e

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	binary       string
	configDir    string
	updateGolden = flag.Bool("update-golden", false, "regenerate golden files")
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Build the binary once
	tmp, err := os.MkdirTemp("", "mailrender-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tmp, "mailrender")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/mailrender")
	cmd.Dir = filepath.Join("..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build failed: " + err.Error())
	}

	// Create test config directory with theme + aerc.conf
	configDir, err = os.MkdirTemp("", "mailrender-config")
	if err != nil {
		panic(err)
	}

	// Write aerc.conf
	os.WriteFile(filepath.Join(configDir, "aerc.conf"), []byte("[ui]\nstyleset-name=test\n"), 0644)

	// Write TOML theme matching the old test palette values
	themesDir := filepath.Join(configDir, "themes")
	os.MkdirAll(themesDir, 0755)
	themeContent := `name = "test"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_secondary" }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
`
	os.WriteFile(filepath.Join(themesDir, "test.toml"), []byte(themeContent), 0644)

	// Copy unwrap-tables.lua into the test config dir so the binary can find it
	filterDir := filepath.Join(configDir, "filters")
	os.MkdirAll(filterDir, 0755)
	luaSrc := filepath.Join("..", ".config", "aerc", "filters", "unwrap-tables.lua")
	luaData, err := os.ReadFile(luaSrc)
	if err != nil {
		panic("reading unwrap-tables.lua: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(filterDir, "unwrap-tables.lua"), luaData, 0644); err != nil {
		panic("writing unwrap-tables.lua: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.RemoveAll(configDir)
	os.Exit(code)
}

func TestHTMLFixtures(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no HTML fixtures found in testdata/")
	}

	for _, fixture := range fixtures {
		name := strings.TrimSuffix(filepath.Base(fixture), ".html")
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			cmd := exec.Command(binary, "html")
			cmd.Stdin = bytes.NewReader(input)
			cmd.Env = append(os.Environ(),
				"AERC_COLUMNS=80",
				"AERC_CONFIG="+configDir,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("running html filter: %v\noutput: %s", err, out)
			}

			goldenPath := filepath.Join("testdata", "golden", name+".txt")
			if *updateGolden {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				if err := os.WriteFile(goldenPath, out, 0644); err != nil {
					t.Fatalf("writing golden file: %v", err)
				}
				return
			}

			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file (run with -update-golden to create): %v", err)
			}
			if !bytes.Equal(out, golden) {
				t.Errorf("output differs from golden file %s\ngot:\n%s\nwant:\n%s", goldenPath, out, golden)
			}
		})
	}
}

func TestHeadersFixture(t *testing.T) {
	input := "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>, Charlie <charlie@example.com>\r\nDate: Mon, 01 Jan 2026 00:00:00 +0000\r\nSubject: Test Message\r\nX-Mailer: test\r\n\r\n"

	cmd := exec.Command(binary, "headers")
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(),
		"AERC_COLUMNS=80",
		"AERC_CONFIG="+configDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running headers filter: %v\noutput: %s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "From:") {
		t.Error("missing From header")
	}
	if !strings.Contains(output, "Subject:") {
		t.Error("missing Subject header")
	}
	if strings.Contains(output, "X-Mailer") {
		t.Error("X-Mailer should be stripped")
	}
}
