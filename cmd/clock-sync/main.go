// Command clock-sync 跑时钟同步样例，输出 JSON；窗口可以用 SYNC_WINDOW_MS 覆盖。
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

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("clock-sync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	window := flags.Int("window", 1000, "采样窗口（毫秒），可用 SYNC_WINDOW_MS 覆盖")
	sample := flags.String("sample", "median", "median / window / monotonic / work")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if env := os.Getenv("SYNC_WINDOW_MS"); env != "" {
		if value, err := strconv.Atoi(env); err == nil {
			*window = value
		}
	}
	encode := func(payload map[string]int) {
		body, _ := json.Marshal(payload)
		fmt.Fprintln(stdout, string(body))
	}
	switch *sample {
	case "median":
		book := clocksync.New(*window, 1)
		for _, offset := range []int{100, 100, 100, 900} {
			book.Add(0, offset, offset, 0, 0)
		}
		value, _, _ := book.Offset(0)
		encode(map[string]int{"offset": value})
	case "window":
		book := clocksync.New(*window, 1)
		book.Add(0, 500, 500, 0, 0)
		book.Add(0, 20, 20, 0, 10000)
		value, _, _ := book.Offset(10000)
		encode(map[string]int{"offset": value})
	case "monotonic":
		book := clocksync.New(100000, 1)
		book.Add(0, 100, 100, 0, 0)
		book.Offset(0)
		book.Add(0, 10, 10, 0, 10)
		value, _, _ := book.Offset(10)
		encode(map[string]int{"offset": value})
	case "work":
		book := clocksync.New(100000, 1)
		for index := 0; index < 3000; index++ {
			book.Add(0, 5, 5, 0, index)
		}
		for index := 0; index < 3000; index++ {
			book.Offset(2999)
		}
		encode(map[string]int{"scanned": book.Scanned()})
	default:
		fmt.Fprintln(stderr, "需要 --sample median|window|monotonic|work")
		return 2
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
