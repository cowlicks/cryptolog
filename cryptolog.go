// Cryptolog is a tool for anonymizing webserver logs.
package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

const (
	ipv4Exp = `(\d\d?\d?\.\d\d?\d?\.\d\d?\d?\.\d\d?\d?)`
	ipv6Exp = `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`
	ipExp   = ipv4Exp + "|" + ipv6Exp
)

var (
	salt = make([]byte, 16)
	mu   sync.Mutex
)

func main() {
	outpath := flag.String("outfile", "", `Destination for encrypted server logs`)
	saltLifetime := flag.Duration("salt-lifetime", time.Hour*24, `Set the lifetime of the hash salt.
This is the duration during which the hashes of a given ip will be identical.
See https://golang.org/pkg/time/#ParseDuration for format.`)
	replaceAll := flag.Bool("replace-all-matches", true, `Replace all occurences of IP addresses within the string.
If false, only the first occurance with be replaced.`)
	flag.Parse()

	go generateSalt(*saltLifetime)

	var out *os.File
	if *outpath != "" {
		var err error
		out, err = os.Create(*outpath)
		if err != nil {
			fmt.Println("Cannot create logfile")
			fmt.Println(err)
			return
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	r := compileRegexp(*replaceAll)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		entry := processSingleLogEntry(scanner.Text(), r)
		fmt.Fprintln(out, entry)
	}
}

func generateSalt(delay time.Duration) {
	for {
		mu.Lock()
		defer mu.Unlock()
		rand.Read(salt)
		time.Sleep(delay)
	}
}

func compileRegexp(replaceAll bool) *regexp.Regexp {
	if replaceAll {
		r, _ := regexp.Compile(ipExp)
		return r
	}
	r, _ := regexp.Compile(`^(.*?)` + ipExp + `(.*)$`)
	return r
}

func processSingleLogEntry(logEntry string, r *regexp.Regexp) string {
	hashedEntry := r.ReplaceAllStringFunc(logEntry, hashIP)
	return hashedEntry
}

func hashIP(ip string) string {
	mu.Lock()
	defer mu.Unlock()
	mac := hmac.New(md5.New, []byte(salt))
	mac.Write([]byte(ip))
	hashedIP := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(hashedIP)[:6]
}
