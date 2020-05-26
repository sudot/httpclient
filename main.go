package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var SplitCompile = regexp.MustCompile("[\\s|\\t]+")
var VariableCompile = regexp.MustCompile("\\{\\{(.+?)}}")

type Environment map[string]string

func main() {
	ph := flag.Bool("h", false, "显示帮助信息")
	pf := flag.String("f", "data\\json.http", "文件路径，若 path 以 #[数字] 结尾(如:/a/b/c.txt#1)则表示只解析此文件中第[数字]个请求")
	pi := flag.Int("i", -1, "请求片段，从 1 开始。优先级比 path 中 #[数字]优先级高")
	pe := flag.String("e", "local", "执行环境")
	flag.Parse()
	h, f, i, e := *ph, *pf, *pi, *pe
	if h {
		flag.Usage()
		return
	}
	if len(f) <= 0 {
		fmt.Print("\n缺少文件路径。可通过 -f 参数录入\n\n\n")
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
	for _, context := range ParseHTTP(f, i) {
		request := NewRequest(context, evn)
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

// 解析出文件中的所有 HTTP 协议格式内容
// 解析后将所有请求内容已字符串数组形式返回
func ParseHTTP(path string, i int) []string {
	paths := RecursiveFile(path)
	splitPrefix := []byte("###")
	comments := [][]byte{
		[]byte("//"),
		[]byte("#"),
	}
	var contents []string
	for _, path = range paths {
		file, _ := os.Open(path)
		// 一行一行的读取,不一次性全部读取
		buf := bufio.NewReader(file)
		line, _, err := buf.ReadLine()
		var content []byte
		for ; err == nil; line, _, err = buf.ReadLine() {
			// 去除前后空格
			s := bytes.TrimSpace(line)
			if bytes.HasPrefix(s, splitPrefix) {
				// 处理请求分段,三个 ###
				// 本次分段内容为空就跳出,无需保留和初始化临时分段数据
				if len(content) > 0 {
					contents = append(contents, string(content))
					content = nil
				}
				if i > 0 {
					l := len(contents)
					if l > 0 {
						if l >= i {
							contents = contents[i-1:]
						} else {
							contents = contents[:0]
						}
						break
					}
				}
				continue
			} else {
				isText := true
				for _, comment := range comments {
					// 寻找注释
					if bytes.HasPrefix(s, comment) {
						isText = false
						break
					}
				}
				if isText {
					// 正常文本行,保留原始内容,不去除前后空格
					content = append(content, line...)
					content = append(content, '\n')
				}
			}
		}
		if len(content) > 0 {
			contents = append(contents, string(content))
		}
		if i > 0 {
			l := len(contents)
			if l > 0 {
				if l >= i {
					contents = contents[i-1:]
				} else {
					contents = contents[:0]
				}
				break
			}
		}
	}
	return contents
}

// 从 HTTP 协议格式内容构造 http.Request 对象实例
func NewRequest(context string, env Environment) *http.Request {
	context = VariableCompile.ReplaceAllStringFunc(context, func(s string) string {
		return env[s[2:len(s)-2]]
	})
	lines := strings.Split(context, "\n")
	// 输出请求地址
	fmt.Println(lines[0])
	split := SplitCompile.Split(lines[0], -1)
	length := len(split)
	if length < 2 {
		panic(fmt.Sprintf("请求行内容格式错误。正确的格式为【请求方法 请求地址 协议版本】。错误数据：【%v】", lines[0]))
	}
	var url string
	var scheme string
	method := split[0]
	protocol := split[length-1]
	_, _, ok := http.ParseHTTPVersion(protocol)
	if ok {
		scheme = strings.ToLower(strings.Split(protocol, "/")[0])
		url = strings.Join(split[1:length-1], "")
	} else {
		scheme = "http"
		url = strings.Join(split[1:], "")
	}

	header := make(http.Header)
	lastHeaderIndex := 0
	for i, line := range lines[1:] {
		index := strings.Index(line, ":")
		if index < 0 {
			// header部分结束
			lastHeaderIndex = i
			break
		}
		key := strings.TrimSpace(line[:index])
		value := strings.TrimSpace(line[index+1:])
		header.Add(key, value)
	}
	if strings.HasPrefix(strings.ToLower(url), scheme) {
		header.Del("Host")
	} else {
		host := header.Get("Host")
		if len(host) > 0 {
			if strings.HasSuffix(host, "/") {
				host = host[:len(host)-1]
			}
			if strings.HasPrefix(url, "/") {
				url = url[1:]
			}
			url = scheme + "://" + host + "/" + url
		}
	}
	_, err := url2.Parse(url)
	if err != nil {
		panic(errors.New(fmt.Sprintf("请求失败。%s", lines[0])))
	}
	bodyIndex := lastHeaderIndex + 2
	var body io.Reader
	if bodyIndex < len(lines) {
		body = bytes.NewBufferString(strings.Join(lines[bodyIndex:], "\n"))
	}
	request, _ := http.NewRequest(method, url, body)
	request.Header = header
	return request
}

func RecursiveFile(path string) []string {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		fmt.Printf("文件不存在. %v", path)
		return nil
	}
	var paths []string
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths
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
func Evn(path string) map[string]Environment {
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
	var evnJson map[string]Environment
	regularFile, _ := os.Open(regularPath)
	defer regularFile.Close()
	_ = json.NewDecoder(regularFile).Decode(&evnJson)
	privateFile, _ := os.Open(privatePath)
	defer privateFile.Close()
	var privateJson map[string]Environment
	_ = json.NewDecoder(privateFile).Decode(&privateJson)
	for k, v := range privateJson {
		m := evnJson[k]
		if m == nil {
			m = make(Environment)
			evnJson[k] = m
		}
		for k1, v1 := range v {
			m[k1] = v1
		}
	}
	return evnJson
}
