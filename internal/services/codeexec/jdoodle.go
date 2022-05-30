package codeexec

import (
	"strings"

	"github.com/ranna-go/ranna/pkg/models"
	"github.com/sarulabs/di/v2"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/static"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/jdoodle"
	"github.com/zekrotja/sop"
)

var langs = []string{"java", "c", "cpp", "c99", "cpp14", "php", "perl", "python3", "ruby", "go", "scala", "bash", "sql", "pascal", "csharp",
	"vbn", "haskell", "objc", "ell", "swift", "groovy", "fortran", "brainfuck", "lua", "tcl", "hack", "rust", "d", "ada", "r", "freebasic",
	"verilog", "cobol", "dart", "yabasic", "clojure", "nodejs", "scheme", "forth", "prolog", "octave", "coffeescript", "icon", "fsharp", "nasm",
	"gccasm", "intercal", "unlambda", "picolisp", "spidermonkey", "rhino", "bc", "clisp", "elixir", "factor", "falcon", "fantom", "pike", "smalltalk",
	"mozart", "lolcode", "racket", "kotlin"}

var specs models.SpecMap = sop.Group[string](sop.Slice(langs), func(v string, _ int) (mk string, mv *models.Spec) {
	mk = v
	mv = &models.Spec{}
	return
})

type JdoodleFactory struct {
	db database.Database
}

var _ Factory = (*JdoodleFactory)(nil)

func NewJdoodleFactory(container di.Container) (e *JdoodleFactory) {
	e = &JdoodleFactory{}

	e.db = container.Get(static.DiDatabase).(database.Database)

	return
}

func (e *JdoodleFactory) Name() string {
	return "jdoodle"
}

func (e *JdoodleFactory) Specs() (models.SpecMap, error) {
	return specs, nil
}

func (e *JdoodleFactory) NewExecutor(guildID string) (exec Executor, err error) {
	jdCreds, err := e.db.GetGuildJdoodleKey(guildID)
	if err != nil || jdCreds == "" {
		return
	}

	jdCredsSplit := strings.Split(jdCreds, "#")
	if len(jdCredsSplit) < 2 {
		return
	}

	exec = &JdoodleExecutor{jdCredsSplit[0], jdCredsSplit[1]}
	return
}

type JdoodleExecutor struct {
	clientId     string
	clientSecret string
}

func (e *JdoodleExecutor) Exec(p Payload) (res Response, err error) {
	w := jdoodle.NewWrapper(e.clientId, e.clientSecret)
	r, err := w.ExecuteScript(p.Language, p.Code)
	if err != nil {
		return
	}

	res.StdOut = r.Output
	res.MemUsed = r.Memory + " Byte"
	res.CpuUsed = r.CPUTime + " Seconds"

	return
}
