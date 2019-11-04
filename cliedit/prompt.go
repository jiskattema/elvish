package cliedit

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"sync"

	"github.com/elves/elvish/cli"
	"github.com/elves/elvish/cli/prompt"
	"github.com/elves/elvish/eval"
	"github.com/elves/elvish/eval/vals"
	"github.com/elves/elvish/eval/vars"
	"github.com/elves/elvish/styled"
	"github.com/elves/elvish/util"
)

func initPrompts(appSpec *cli.AppSpec, nt notifier, ev *eval.Evaler, ns eval.Ns) {
	promptVal, rpromptVal := getDefaultPromptVals()
	initPrompt(&appSpec.Prompt, "prompt", promptVal, nt, ev, ns)
	initPrompt(&appSpec.RPrompt, "rprompt", rpromptVal, nt, ev, ns)
}

func initPrompt(p *cli.Prompt, name string, val eval.Callable, nt notifier, ev *eval.Evaler, ns eval.Ns) {
	theVar := vars.FromPtr(&val)
	ns[name] = theVar
	*p = prompt.New(func() styled.Text {
		return callPrompt(nt, ev, theVar.Get().(eval.Callable))
	})
}

func getDefaultPromptVals() (prompt, rprompt eval.Callable) {
	user, userErr := user.Current()
	isRoot := userErr == nil && user.Uid == "0"

	username := "???"
	if userErr == nil {
		username = user.Username
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "???"
	}

	return getDefaultPrompt(isRoot), getDefaultRPrompt(username, hostname)
}

func getDefaultPrompt(isRoot bool) eval.Callable {
	p := styled.Plain("> ")
	if isRoot {
		p = styled.Transform(styled.Plain("# "), "red")
	}
	return eval.NewGoFn("default prompt", func(fm *eval.Frame) styled.Text {
		return styled.Plain(util.Getwd()).ConcatText(p)
	})
}

func getDefaultRPrompt(username, hostname string) eval.Callable {
	rp := styled.Transform(styled.Plain(username+"@"+hostname), "inverse")
	return eval.NewGoFn("default rprompt", func() styled.Text {
		return rp
	})
}

// callPrompt calls a function with no arguments and closed input, and converts
// its outputs to styled objects. Used to call prompt callbacks.
func callPrompt(nt notifier, ev *eval.Evaler, fn eval.Callable) styled.Text {
	ports := []*eval.Port{
		eval.DevNullClosedChan,
		{}, // Will be replaced when capturing output
		{File: os.Stderr},
	}

	return callForStyledText(nt, ev, fn, ports)
}

func callForStyledText(nt notifier, ev *eval.Evaler, fn eval.Callable, ports []*eval.Port) styled.Text {

	var (
		result      styled.Text
		resultMutex sync.Mutex
	)
	add := func(v interface{}) {
		resultMutex.Lock()
		defer resultMutex.Unlock()
		newResult, err := result.Concat(v)
		if err != nil {
			nt.Notify(fmt.Sprintf(
				"invalid output type from prompt: %s", vals.Kind(v)))
		} else {
			result = newResult.(styled.Text)
		}
	}

	// Value outputs are concatenated.
	valuesCb := func(ch <-chan interface{}) {
		for v := range ch {
			add(v)
		}
	}
	// Byte output is added to the prompt as a single unstyled text.
	bytesCb := func(r *os.File) {
		allBytes, err := ioutil.ReadAll(r)
		if err != nil {
			nt.Notify(fmt.Sprintf("error reading prompt byte output: %v", err))
		}
		if len(allBytes) > 0 {
			add(string(allBytes))
		}
	}

	// XXX There is no source to pass to NewTopEvalCtx.
	fm := eval.NewTopFrame(ev, eval.NewInternalSource("[prompt]"), ports)
	err := fm.CallWithOutputCallback(fn, nil, eval.NoOpts, valuesCb, bytesCb)

	if err != nil {
		nt.Notify(fmt.Sprintf("prompt function error: %v", err))
		return nil
	}

	return result
}
