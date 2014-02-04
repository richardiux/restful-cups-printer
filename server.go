package main

import (
	"bytes"
	"fmt"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

const version = "0.1.1"

func printDocument(res render.Render, req *http.Request, w http.ResponseWriter) {
	fetchDocument := func(url string) *os.File {
		os.Mkdir("./tmp", 0700)

		out, _ := ioutil.TempFile("tmp", "document")
		defer out.Close()

		resp, _ := http.Get(url)
		defer resp.Body.Close()

		io.Copy(out, resp.Body)

		return out
	}

	removeDocument := func(name string) error {
		return os.Remove(name)
	}

	sendToPrinter := func(documentPath string, printer string, orientation string, media string) ([]string, string) {
		var printOptions []string

		if printer != "" {
			printOptions = append(printOptions, "-P", printer)
		}
		if orientation != "" {
			printOptions = append(printOptions, "-o", orientation)
		}
		if media != "" {
			printOptions = append(printOptions, "-o", fmt.Sprintf("media=%s", media))
		}
		printOptions = append(printOptions, documentPath)

		cmd := exec.Command("lpr", printOptions...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Run()
		return printOptions, out.String()
	}

	qs := req.URL.Query()
	printer, orientation, media, url := qs.Get("printer"), qs.Get("orientation"), qs.Get("media"), qs.Get("path")

	document := fetchDocument(url)
	printOptions, printOut := sendToPrinter(document.Name(), printer, orientation, media)
	removeDocument(document.Name())

	w.Header().Set("Access-Control-Allow-Origin", "*")

	res.JSON(200, map[string]interface{}{
		"printer":       printer,
		"orientation":   orientation,
		"media":         media,
		"url":           url,
		"tmp_file":      document.Name(),
		"print_options": printOptions,
		"print_ouput":   printOut,
	})
}

func cancelAll(res render.Render, req *http.Request, w http.ResponseWriter) {
	qs := req.URL.Query()
	printer := qs.Get("printer")

	cmd := exec.Command("cancel", "-a", "-x", printer)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()

	w.Header().Set("Access-Control-Allow-Origin", "*")

	res.JSON(200, map[string]interface{}{
		"success": true,
		"printer": printer,
	})
}

func currentUser() (s string) {
	u, _ := user.Current()
	return u.Username
}

func main() {
	m := martini.Classic()
	m.Use(render.Renderer())
	m.Use(martini.Logger())

	m.Get("/", func() string {
		return "<h1>Receipt Printer</h1>"
	})

	m.Get("/cache/clear", func(res render.Render, w http.ResponseWriter) {
		os.RemoveAll(filepath.Join("./tmp/"))
		w.Header().Set("Access-Control-Allow-Origin", "*")
		res.JSON(200, map[string]interface{}{"success": true})
	})

	m.Get("/print", printDocument)

	m.Get("/cancel/all", cancelAll)

	m.Get("/status", func(res render.Render, w http.ResponseWriter) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		res.JSON(200, map[string]interface{}{"success": true, "version": version})
	})

	if err := http.ListenAndServeTLS(":9631", "cert.pem", "key.pem", m); err != nil {
		log.Fatal(err)
	}

	m.Run()
}
