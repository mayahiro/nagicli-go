package document_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cli "github.com/mayahiro/nagicli-go"
	"github.com/mayahiro/nagicli-go/document"
)

func TestRenderersMatchSharedGoldenFiles(t *testing.T) {
	help := fixtureDocument(t)
	for name, actual := range map[string]string{
		"markdown.txt": (document.MarkdownRenderer{}).Render(help),
		"man.txt":      (document.ManRenderer{}).Render(help),
	} {
		expected, err := os.ReadFile(filepath.Join(fixtureRoot(t), "cli", "document", name))
		if err != nil {
			t.Fatal(err)
		}
		if actual != string(expected) {
			t.Fatalf("%s differs\ngot:\n%s\nwant:\n%s", name, actual, expected)
		}
	}
}

func TestGeneratedTextCannotInjectMarkdownOrRoffStructure(t *testing.T) {
	help := fixtureDocument(t)
	markdown := (document.MarkdownRenderer{}).Render(help)
	if !strings.Contains(markdown, `\.SH ATTACK`) || strings.Contains(markdown, "\n.SH ATTACK\n") {
		t.Fatalf("unsafe Markdown:\n%s", markdown)
	}
	if strings.TrimRight(markdown, "\n")+"\n" != markdown {
		t.Fatalf("Markdown final newline = %q", markdown[len(markdown)-2:])
	}

	man := (document.ManRenderer{}).Render(help)
	if !strings.Contains(man, "\n\\&.SH ATTACK\n") ||
		!strings.Contains(man, "\n\\&'quoted\n") ||
		strings.Contains(man, "\n.SH ATTACK\n") {
		t.Fatalf("unsafe man page:\n%s", man)
	}
	if strings.TrimRight(man, "\n")+"\n" != man {
		t.Fatalf("man final newline = %q", man[len(man)-2:])
	}
}

func TestRepeatedDerivedDocumentsDoNotRetainOutputs(t *testing.T) {
	command := benchmarkGraph(1_000)
	renderWindow := func() {
		for range 32 {
			err := command.VisitHelpDocuments(func(help cli.HelpDocument) bool {
				markdown := (document.MarkdownRenderer{}).Render(help)
				man := (document.ManRenderer{}).Render(help)
				runtime.KeepAlive(markdown)
				runtime.KeepAlive(man)
				return true
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	renderWindow()
	baseline := heapAllocationAfterGC()
	renderWindow()
	retained := heapAllocationAfterGC()
	const tolerance = uint64(1 << 20)
	if retained > baseline+tolerance {
		t.Fatalf("retained heap grew from %d to %d bytes", baseline, retained)
	}
	runtime.KeepAlive(command)
}

func fixtureDocument(t *testing.T) cli.HelpDocument {
	t.Helper()
	help, err := fixtureCommand().HelpDocument([]string{"qed", "run"})
	if err != nil {
		t.Fatal(err)
	}
	return help
}

func fixtureCommand() *cli.Command {
	return cli.NewCommand("qed").
		About("Workspace manager").
		Version("1.0.0").
		Option(
			cli.ValueOption("config").
				Long("config").
				Help("Configuration [path]").
				Default("top-secret").
				Sensitive().
				Inherited(),
		).
		Subcommand(
			cli.NewCommand("run").
				About("Run *one* task\n.SH ATTACK").
				Deprecated("execute").
				Argument(
					cli.Positional("target").
						Help("Target <name>").
						Required().
						Sensitive(),
				).
				Option(
					cli.ValueOption("profile").
						Long("profile").
						Help("Profile `name`").
						Default("prod").
						Deprecated("context"),
				).
				Option(
					cli.Flag("force").
						Long("force").
						Help("Force - carefully").
						RequiresSupplied("profile"),
				).
				Option(cli.Flag("json").Long("json").Help("JSON output")).
				Option(cli.Flag("yaml").Long("yaml").Help("YAML output")).
				OptionGroup(cli.AtMostOne("format", "json", "yaml")).
				Subcommand(cli.NewCommand("inspect").About("Inspect result")).
				Example("Run `once`", "qed run --profile dev target").
				Note("'quoted\n.danger\n    indented\nUse # carefully").
				Link("Guide [stable]", "https://example.test/a path?q=(x)<unsafe>").
				HelpSection(
					cli.NewHelpSection("details", "More *details*").
						Paragraph("Backslash \\ and - dash\n'quoted").
						Entry(".macro", "Value | table"),
				),
		).
		Subcommand(
			cli.NewCommand("secret").
				Hidden().
				Subcommand(cli.NewCommand("hidden-descendant")),
		)
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("NAGI_FIXTURES")
	if root == "" {
		root = filepath.Join("..", "..", "fixtures")
	}
	return root
}

func heapAllocationAfterGC() uint64 {
	runtime.GC()
	var statistics runtime.MemStats
	runtime.ReadMemStats(&statistics)
	return statistics.HeapAlloc
}
