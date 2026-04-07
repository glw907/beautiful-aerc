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
	paletteDir   string
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

	// Create a test palette so the binary can load colors
	paletteDir, err = os.MkdirTemp("", "mailrender-palette")
	if err != nil {
		panic(err)
	}
	genDir := filepath.Join(paletteDir, "generated")
	os.MkdirAll(genDir, 0755)
	palette := `BG_BASE=#2e3440
BG_ELEVATED=#3b4252
BG_SELECTION=#394353
BG_BORDER=#49576b
FG_BASE=#d8dee9
FG_BRIGHT=#e5e9f0
FG_BRIGHTEST=#eceff4
FG_DIM=#616e88
ACCENT_PRIMARY=#81a1c1
ACCENT_SECONDARY=#88c0d0
ACCENT_TERTIARY=#8fbcbb
COLOR_ERROR=#bf616a
COLOR_WARNING=#d08770
COLOR_SUCCESS=#a3be8c
COLOR_INFO=#ebcb8b
COLOR_SPECIAL=#b48ead
C_HEADING="1;38;2;163;190;140"
C_BOLD="1"
C_ITALIC="3"
C_LINK_TEXT="38;2;136;192;208"
C_LINK_URL="38;2;97;110;136"
C_RULE="38;2;97;110;136"
C_RESET="0"
C_HDR_KEY="38;2;129;161;193;1"
C_HDR_VALUE="38;2;216;222;233"
C_HDR_DIM="38;2;97;110;136"
C_PICKER_NUM="38;2;129;161;193"
C_PICKER_LABEL="38;2;216;222;233"
C_PICKER_URL="38;2;97;110;136"
C_PICKER_SEL_BG="38;2;57;67;83"
C_PICKER_SEL_FG="38;2;229;233;240"
C_MSG_MARKER="38;2;97;110;136;1"
C_MSG_TITLE_SUCCESS="38;2;163;190;140;1"
C_MSG_TITLE_ACCENT="38;2;129;161;193;1"
C_MSG_DETAIL="38;2;216;222;233"
C_MSG_DIM="38;2;97;110;136"
`
	os.WriteFile(filepath.Join(genDir, "palette.sh"), []byte(palette), 0644)

	// Copy unwrap-tables.lua into the test palette dir so the binary can find it
	filterDir := filepath.Join(paletteDir, "filters")
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
	os.RemoveAll(paletteDir)
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
				"AERC_CONFIG="+paletteDir,
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
		"AERC_CONFIG="+paletteDir,
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
