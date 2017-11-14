// +build !windows

package rewriter

import (
	"fmt"
	"strings"
)

func (w *Rewriter) clearLines() {
	fmt.Fprint(w.out, strings.Repeat(clearCursorAndLine, w.lineCount))
}
