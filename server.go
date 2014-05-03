package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
)

const version = "0.1.4"

func printDocument(res render.Render, req *http.Request, w http.ResponseWriter) {
	fetchDocument := func(url string) (*os.File, error) {
		if url == "" {
			err := fmt.Errorf("URL is blank")
			return nil, err
		}

		os.Mkdir("./tmp", 0700)

		out, _ := ioutil.TempFile("tmp", "document")
		defer out.Close()

		resp, err := http.Get(url)
		defer resp.Body.Close()

		io.Copy(out, resp.Body)

		return out, err
	}

	removeDocument := func(name string) error {
		return os.Remove(name)
	}

	sendToPrinter := func(url string, printer string, orientation string, media string) ([]string, string, error) {
		document, err := fetchDocument(url)
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		defer removeDocument(document.Name())

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
		printOptions = append(printOptions, document.Name())

		cmd := exec.Command("lpr", printOptions...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		return printOptions, out.String(), nil
	}

	qs := req.URL.Query()
	printer, orientation, media, url := qs.Get("printer"), qs.Get("orientation"), qs.Get("media"), qs.Get("path")

	go sendToPrinter(url, printer, orientation, media)

	w.Header().Set("Access-Control-Allow-Origin", "*")

	res.JSON(200, map[string]interface{}{
		"status": "scheduled",
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

	m.Get("/self_update", func(res render.Render, w http.ResponseWriter) {
		cmd := exec.Command("./update")
		cmd.Run()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		res.JSON(200, map[string]interface{}{"success": true, "version": version})
	})

	m.Get("/status", func(res render.Render, w http.ResponseWriter) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		res.JSON(200, map[string]interface{}{"success": true, "version": version})
	})

	go func() {
		if err := http.ListenAndServe(":9632", m); err != nil {
			log.Fatal(err)
		}
	}()

	if err := http.ListenAndServeTLS(":9631", "cert.pem", "key.pem", m); err != nil {
		log.Fatal(err)
	}

	m.Run()
}
