// Command clock-sync 跑时钟校正样例，输出一行 JSON；窗口可用 SYNC_WINDOW_MS 覆盖。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"example.com/clocksync/clocksync"
)

func emit(stdout io.Writer, payload map[string]int) {
	body, _ := json.Marshal(payload)
	fmt.Fprintln(stdout, string(body))
}

// handshake 用 (t1,t2,t3,t4) 造一次偏移等于 offset 的握手。
func handshake(book *clocksync.Sync, source string, offset, atMs int) {
	book.Add(source, 0, offset, offset, 0, atMs)
}

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("clock-sync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	window := flags.Int("window", 1000, "采样窗口（毫秒），可用 SYNC_WINDOW_MS 覆盖")
	sample := flags.String("sample", "median", "median / window / jump / budget / work")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if env := os.Getenv("SYNC_WINDOW_MS"); env != "" {
		if value, err := strconv.Atoi(env); err == nil {
			*window = value
		}
	}
	switch *sample {
	case "median":
		book := clocksync.New(*window, 1, 100000, 100000)
		for _, offset := range []int{100, 100, 100, 900} {
			handshake(book, "s1", offset, 0)
		}
		value, _, _ := book.Offset("s1", 0)
		emit(stdout, map[string]int{"offset": value})
	case "window":
		book := clocksync.New(*window, 1, 100000, 100000)
		handshake(book, "s1", 500, 0)
		handshake(book, "s1", 20, 10000)
		value, _, _ := book.Offset("s1", 10000)
		emit(stdout, map[string]int{"offset": value})
	case "jump":
		book := clocksync.New(100000, 1, 500, 100000)
		handshake(book, "s1", 100, 0)
		book.Tick("s1", 0)
		book.DropSamples("s1")
		handshake(book, "s1", 1500, 10)
		_, ok := book.Tick("s1", 10)
		emit(stdout, map[string]int{"ok": boolToInt(ok), "blocked": book.Blocked})
	case "budget":
		book := clocksync.New(100000, 1, 100000, 100)
		handshake(book, "s1", 80, 0)
		book.Tick("s1", 0)
		book.DropSamples("s1")
		handshake(book, "s1", 160, 10)
		delta, _ := book.Tick("s1", 10)
		emit(stdout, map[string]int{"second": delta, "deferred": book.Deferred})
	case "work":
		book := clocksync.New(100000, 1, 100000, 100000)
		for index := 0; index < 3000; index++ {
			handshake(book, "s1", 5, index)
		}
		for index := 0; index < 3000; index++ {
			book.Offset("s1", 2999)
		}
		emit(stdout, map[string]int{"scanned": book.Scanned()})
	default:
		fmt.Fprintln(stderr, "需要 --sample median|window|jump|budget|work")
		return 2
	}
	return 0
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
