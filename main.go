package main

//LUFAX

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type LogReader struct {
	Logfile    string
	Namespace  string
	Appname    string
	Keyword    string
	Starttime  string
	Endtime    string
	Linenumber int
	Report     bool
	Apiserver  string
	//Lineclosenumber int
}

var (
	help      bool
	report    bool
	app       string
	namespace string
	keyword   string
	starttime string
	endtime   string
	file      string
	apiserver string
	line      int
	ISOTIME   = "2006-01-02 15:04:05"
	logpath   = "/wls/applications/%s/logs/%s.log"
)

func argsInit() {
	flag.IntVar(&line, "l", 10, "search logfile lines after keyword")
	flag.BoolVar(&help, "h", false, "help output")
	flag.BoolVar(&report, "r", false, "report to api server")
	flag.StringVar(&namespace, "n", "", "announce namespace env")
	flag.StringVar(&app, "a", "", "announce app env")
	flag.StringVar(&keyword, "k", "", "search keyword in logfile")
	flag.StringVar(&starttime, "s", "", "set the begining of the query,it rely on endtime")
	flag.StringVar(&endtime, "e", "", "set the ending of the query,it rely on starttime")
	flag.StringVar(&file, "f", "", "assign log file")
	flag.StringVar(&apiserver, "api", "glog.bobtthp.com", "api server addr string")
	flag.Usage = usage
}

func usage() {
	fmt.Fprintf(os.Stderr, `glog version: glog/1.0.0
Usage: glog [-h help] [-a app] [-n namespace] [-f file] [-r report] [-k keyword] [-s starttime] [-e endtime] [-l line]

Options:
`)
	flag.PrintDefaults()
}

func (glog *LogReader) gologInit(a string, f string, n string, k string, s string, e string, l int, r bool, api string) {
	glog.Appname = a
	glog.Endtime = e
	glog.Starttime = s
	glog.Keyword = k
	glog.Linenumber = l
	glog.Namespace = n
	glog.Report = r
	glog.Apiserver = api
	if f == "" {
		nowDate := time.Now().Format(ISOTIME)[0:10]
		logDate := e[0:10]

		//fmt.Println(nowDate, logDate)
		if nowDate != logDate {
			f := fmt.Sprintf(logpath, a, a+".log."+logDate)
			glog.Logfile = f
		} else {
			f = fmt.Sprintf(logpath, a, a)
			glog.Logfile = f
		}
	} else {
		glog.Logfile = f
	}

}

func (glog *LogReader) logPathCheck() bool {
	f, err := os.Stat(glog.Logfile)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		fmt.Println(err.Error())
		return false
	} else if f.IsDir() {
		fmt.Printf("%s is dir")
		return false
	} else {
		fmt.Println(err.Error())
		return false
	}
}

func (glog *LogReader) timeCompare() bool {
	start, err := time.Parse(ISOTIME, glog.Starttime)
	if err != nil {
		fmt.Println(err.Error())
	}
	end, err := time.Parse(ISOTIME, glog.Endtime)
	if err != nil {
		fmt.Println(err.Error())
	}
	if start.Unix() > end.Unix() {
		return false
	} else {
		return true
	}
}

func ReadFile(filePath string, linenum int, keyword string, starttime string, endtime string) (result string) {
	var linenum_tmp int
	start_timestamp, _ := time.Parse(ISOTIME, starttime)
	end_timestamp, _ := time.Parse(ISOTIME, endtime)

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		fmt.Println(err.Error())
	}

	buf := bufio.NewReader(f)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			//fmt.Println(err.Error())
			//EXIT READ FILE
			break
		}
		//FILTER

		//GET LOG TIME FOR PARSE
		timestamp := line[0:19]
		line_timestamp, err := time.Parse(ISOTIME, timestamp)
		if err != nil {
			fmt.Println("log format is not support !")
			break
		}
		//FILTER TIME
		if start_timestamp.Unix() <= line_timestamp.Unix() && end_timestamp.Unix() >= line_timestamp.Unix() {
			//FILTER KEYWORD
			if strings.Contains(line, keyword) {
				linenum_tmp = linenum
				newKeyword := fmt.Sprintf("%c[1;31;40m%s%c[0m", 0x1B, keyword, 0x1B)

				re, err := regexp.Compile(keyword)
				if err != nil {
					fmt.Println(err.Error())
				}

				n := re.ReplaceAllString(line, newKeyword)
				result += n
				//GOTO NEXT LOOP
				continue
			}
			//ADD OTHER LINE
			if linenum_tmp <= linenum && linenum_tmp > 0 {
				linenum_tmp = linenum_tmp - 1
				result += line
			}
		}
	}
	return result
}

func (glog *LogReader) logRead() (filesize string, logs string) {
	logsize := getLogSize(glog.Logfile)
	filesize = fmt.Sprintf("%dK", logsize)
	logs = ReadFile(glog.Logfile, glog.Linenumber, glog.Keyword, glog.Starttime, glog.Endtime)
	return filesize, logs
}

func (glog *LogReader) sendToServer(logsize string, loginfo string) bool {
	var logInfoMap map[string]interface{}
	logInfoMap = make(map[string]interface{})
	logInfoMap["logfile"] = glog.Logfile
	logInfoMap["appname"] = glog.Appname
	logInfoMap["zone"] = glog.Namespace
	logInfoMap["logsize"] = logsize
	logInfoMap["loginfo"] = loginfo

	jsonData, err := json.Marshal(logInfoMap)

	if err != nil {

		fmt.Println("parse json err", logInfoMap, err.Error())
		return false
	}

	resp, err := http.Post("http://"+glog.Apiserver,
		"application/json",
		bytes.NewBuffer(jsonData))

	if err != nil {
		fmt.Println("post api server err", glog.Apiserver, err.Error())

		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
	}

	if resp.StatusCode == 200 {
		fmt.Println(string(body))
		return true
	} else {
		return false
	}

}

func getLogSize(path string) (size int64) {
	f, err := os.Stat(path)
	if err != nil {
		fmt.Println(err.Error())
	}
	//return MB
	return f.Size() / 1024
}

func console(width int) {
	for l := 0; l < width; l++ {
		fmt.Printf("#")
	}
	fmt.Printf("\n")
}

func consoleOutput(appname string, logsize string, logfile string, zone string, info interface{}) {
	console(120)
	console(120)
	fmt.Println("")
	fmt.Println()
	fmt.Println("appname: ", appname)
	fmt.Println("zone :", zone)
	fmt.Println("logsize :", logsize)
	fmt.Println("logfile :", logfile)
	fmt.Printf("loginfo :")
	fmt.Println(info)
	fmt.Println("")
	console(120)
	console(120)
}

func main() {
	//init flag
	argsInit()
	//parse args
	flag.Parse()

	//main start
	if help {
		flag.Usage()
	} else if app != "" && namespace != "" && keyword != "" && starttime != "" && endtime != "" && line >= 0 {
		// args init
		var glog LogReader
		glog.gologInit(app, file, namespace, keyword, starttime, endtime, line, false, apiserver)
		//fmt.Println(glog)
		// args check
		if glog.timeCompare() && glog.logPathCheck() {
			//log read
			size, logs := glog.logRead()
			if report {
				glog.Report = true
				consoleOutput(glog.Appname, size, glog.Logfile, glog.Namespace, logs)
				glog.sendToServer(size, logs)
				fmt.Println("send report to api", glog.Apiserver)
			} else {
				fmt.Println("don't send report to api")
			}
		} else {
			fmt.Println("endtime must greater than starttime,or logpath don't exist")
		}

	} else {
		flag.Usage()
	}
}
