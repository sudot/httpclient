package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/sudot/httpclient/parse"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ph *bool
	pf *string
	pi *int
	pe *string
)

func init() {
	ph = flag.Bool("h", false, "显示帮助信息")
	pf = flag.String("f", "", "文件路径，若 path 以 #[数字] 结尾(如:/a/b/c.txt#1)则表示只解析此文件中第[数字]个请求")
	pi = flag.Int("i", -1, "请求片段，从 1 开始。优先级比 path 中 #[数字]优先级高")
	pe = flag.String("e", "", "执行环境")
}

func main() {
	flag.Parse()
	h, f, i, e := *ph, *pf, *pi, *pe
	//f = "data\\github.http"
	//i = 2
	//e = "local"
	if h {
		flag.Usage()
		return
	}
	if len(f) <= 0 {
		fmt.Print("\n缺少文件路径。可通过 -f 参数录入\n\n")
		flag.Usage()
		return
	}
	if len(e) <= 0 {
		fmt.Print("\n缺少执行环境。可通过 -e 参数录入\n\n")
		flag.Usage()
		return
	}
	index := strings.LastIndex(f, "#")
	if index > 0 {
		fi, err := strconv.Atoi((f)[index+1:])
		if i < 1 && err == nil {
			i = fi
		}
		f = f[0:index]
	}
	evn := Evn(".")[e]
	client := http.DefaultClient
	for _, context := range parse.ParseHTTP(f, i) {
		request := parse.NewRequest(context, evn)
		if request == nil {
			continue
		}
		resp, err := client.Do(request)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		var reader = resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, _ = gzip.NewReader(resp.Body)
		}
		body, _ := ioutil.ReadAll(reader)
		_ = resp.Body.Close()

		fmt.Printf("\n%s %v\n", resp.Proto, resp.Status)
		for k, v := range resp.Header {
			fmt.Printf("%s: %s\n", k, strings.Join(v, ","))
		}
		fmt.Printf("\n%s\n\n", body)
	}
}

// 递归寻找指定目录下所有目录中的 http-client.env.json 和 http-client.private.env.json 文件
// 并读取找到的第一个文件内容作为环境变量
// 文件格式
// {
//  "dev": {
//    "name": "value"
//  },
//  "test": {
//    "name": "value"
//  }
// }
func Evn(path string) map[string]parse.Environment {
	var (
		regular     = "http-client.env.json"
		private     = "http-client.private.env.json"
		regularPath = ""
		privatePath = ""
	)
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		rl := len(regularPath)
		pl := len(privatePath)
		if rl <= 0 && strings.EqualFold(info.Name(), regular) {
			regularPath = path
		}
		if pl <= 0 && strings.EqualFold(info.Name(), private) {
			privatePath = path
		}
		if rl > 0 && pl > 0 {
			// 结束遍历
			return errors.New("over")
		}
		return nil
	})
	var evnJson map[string]parse.Environment
	regularFile, _ := os.Open(regularPath)
	defer regularFile.Close()
	_ = json.NewDecoder(regularFile).Decode(&evnJson)
	privateFile, _ := os.Open(privatePath)
	defer privateFile.Close()
	var privateJson map[string]parse.Environment
	_ = json.NewDecoder(privateFile).Decode(&privateJson)
	for k, v := range privateJson {
		m := evnJson[k]
		if m == nil {
			m = make(parse.Environment)
			evnJson[k] = m
		}
		for k1, v1 := range v {
			m[k1] = v1
		}
	}
	return evnJson
}
