package banner

import (
	"fmt"
	"io"
	"os"
	"time"
)

// StartupOpts allows tests to capture output and disable delays.
// If nil, Startup uses os.Stdout and default animation delays.
type StartupOpts struct {
	Writer   io.Writer     // if set, use instead of os.Stdout
	NoDelay  bool         // if true, do not sleep between lines or at end
}

// Banner ASCII art (IRONCLAW).
const bannerArt = `
                                                                                
@@@  @@@@@@@    @@@@@@   @@@  @@@   @@@@@@@  @@@        @@@@@@   @@@  @@@  @@@  
@@@  @@@@@@@@  @@@@@@@@  @@@@ @@@  @@@@@@@@  @@@       @@@@@@@@  @@@  @@@  @@@  
@@!  @@!  @@@  @@!  @@@  @@!@!@@@  !@@       @@!       @@!  @@@  @@!  @@!  @@!  
!@!  !@!  @!@  !@!  @!@  !@!!@!@!  !@!       !@!       !@!  @!@  !@!  !@!  !@!  
!!@  @!@!!@!   @!@  !@!  @!@ !!@!  !@!       @!!       @!@!@!@!  @!!  !!@  @!@  
!!!  !!@!@!    !@!  !!!  !@!  !!!  !!!       !!!       !!!@!!!!  !@!  !!!  !@!  
!!:  !!: :!!   !!:  !!!  !!:  !!!  :!!       !!:       !!:  !!!  !!:  !!:  !!:  
:!:  :!:  !:!  :!:  !:!  :!:  !:!  :!:        :!:      :!:  !:!  :!:  :!:  :!:  
 ::  ::   :::  ::::: ::   ::   ::   ::: :::   :: ::::  ::   :::   :::: :: :::   
:     :   : :   : :  :   ::    :    :: :: :  : :: : :   :   : :    :: :  : :
`

// Startup prints the ASCII banner with a short animation, then the version line.
// If opts is non-nil and opts.Writer is set, output goes there; if opts.NoDelay is true, animation delays are skipped.
func Startup(version string, opts *StartupOpts) {
	w := io.Writer(os.Stdout)
	lineDelay := 35 * time.Millisecond
	endDelay := 150 * time.Millisecond
	if opts != nil {
		if opts.Writer != nil {
			w = opts.Writer
		}
		if opts.NoDelay {
			lineDelay = 0
			endDelay = 0
		}
	}
	if w == os.Stdout {
		fmt.Print("\033[2J\033[H\033[?25l") // clear, home, hide cursor
		defer fmt.Print("\033[?25h")         // show cursor on exit
	}

	// Typewriter-style: print banner line by line
	lines := splitLines(bannerArt)
	for _, line := range lines {
		fmt.Fprintln(w, line)
		if lineDelay > 0 {
			time.Sleep(lineDelay)
		}
	}
	fmt.Fprintf(w, "\033[36m  agent framework  \033[0m  v%s\n", version)
	if endDelay > 0 {
		time.Sleep(endDelay)
	}
	fmt.Fprintln(w)
}

func splitLines(s string) []string {
	var out []string
	var line []rune
	for _, r := range s {
		if r == '\n' {
			if len(line) > 0 || out != nil {
				out = append(out, string(line))
			}
			line = line[:0]
			continue
		}
		line = append(line, r)
	}
	if len(line) > 0 {
		out = append(out, string(line))
	}
	return out
}
