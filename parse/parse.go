package parse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"regexp"
	"strings"
)

// 解析出文件中的所有 HTTP 协议格式内容
// 在去除前后空格的行中，以连续的 3 个 # 符号开头的为分段行
// 解析后将所有请求内容已字符串数组形式返回
// 若给定了 i 的值，且大于 0，则只返回 i 所对应的一个请求
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
		readLine, _, err := buf.ReadLine()
		var lines []byte
		for ; err == nil; readLine, _, err = buf.ReadLine() {
			// 去除前后空格
			s := bytes.TrimSpace(readLine)
			if bytes.HasPrefix(s, splitPrefix) {
				// 处理请求分段,三个 ###
				// 本次分段内容为空就跳出,无需保留和初始化临时分段数据
				if len(lines) > 0 {
					contents = append(contents, string(lines))
					lines = nil
				}
				if i > 0 && len(contents) >= i {
					break
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
					lines = append(lines, readLine...)
					lines = append(lines, '\n')
				}
			}
		}
		if len(lines) > 0 {
			contents = append(contents, string(lines))
		}
		if i > 0 && len(contents) >= i {
			contents = contents[i-1 : i]
			break
		}
	}
	return contents
}

type Environment map[string]string

var SplitCompile = regexp.MustCompile("[\\s|\\t]+")
var VariableCompile = regexp.MustCompile("\\{\\{(.+?)}}")

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
