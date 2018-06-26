package pdownload

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
)

//Download download file from <urlStr> to <filepath> use <concurrency> goroutines
func Download(urlStr string, filepath string, concurrency int) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	resp, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	contentLength := resp.ContentLength
	amountPerConnection := contentLength / int64(concurrency)

	prefix := filepath + "part"
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var lastbit int64
	var endbit int64
	lastbit = 0

	for i := 0; i < concurrency; i++ {
		endbit = lastbit + amountPerConnection
		go downloadPart(parsedURL, lastbit, endbit, prefix, i, &wg)
		lastbit = endbit + 1
	}
	wg.Wait()
	os.Remove(filepath)
	mergeFiles(prefix, concurrency, filepath)
	for i := 0; i < concurrency; i++ {
		prefixfilename := prefix + "." + strconv.Itoa(i)
		os.Remove(prefixfilename)
	}
	return nil
}

func mergeFiles(prefix string, parts int, outfile string) {
	out, _ := os.Create(outfile)
	for i := 0; i < parts; i++ {
		filename := prefix + "." + strconv.Itoa(i)
		in, _ := os.Open(filename)
		io.Copy(out, in)
		in.Close()
	}
}

func downloadPart(
	durl *url.URL, startbit int64, endbit int64,
	prefix string, part int, wg *sync.WaitGroup,
) {
	defer wg.Done()
	filename := prefix + "." + strconv.Itoa(part)
	req, _ := http.NewRequest("GET", durl.String(), nil)
	rangeHeader := fmt.Sprintf("bytes=%d-%d", startbit, endbit)
	req.Header.Add("Range", rangeHeader)
	requestToFile(req, filename)
}

func requestToFile(req *http.Request, filename string) error {
	os.Remove(filename)
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()
	client := http.Client{}
	attempts := 4
	var responseErr error
	responseErr = nil
	for attempts > 0 {
		attempts--
		resp, err := client.Do(req)
		responseErr = err
		if err != nil {
			continue
		}
		io.Copy(out, resp.Body)
		resp.Body.Close()
		return nil
	}
	return responseErr
}
