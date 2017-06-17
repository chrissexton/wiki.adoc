// © 2017 the wiki.adoc Authors under the WTFPL license. See AUTHORS for the list of authors.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	inpath       = flag.String("path", "./", "Path to the root of your wiki src")
	outPath      = flag.String("out", "./html", "Path to the output")
	pdf          = flag.Bool("pdf", false, "Render PDFs")
	excludeList  = flag.String("exclude", "html,.git", "Exclusion comma separated list of files")
	mediaList    = flag.String("media", "img,resources", "comma separated paths designated as media (to be copied)")
	mediaNames   = make([]string, 0)
	excludeNames = make([]string, 0)
	requires     = reqList{}

	indexes = make(map[string][]string)
)

type reqList []string

func (r *reqList) Set(val string) error {
	*r = append(*r, val)
	return nil
}

func (r *reqList) String() string {
	return fmt.Sprint(*r)
}

func main() {
	flag.Var(&requires, "r", "Added require flags")
	flag.Parse()

	excludeNames = strings.Split(*excludeList, ",")
	mediaNames = strings.Split(*mediaList, ",")

	*outPath = filepath.Clean(*outPath)

	err := filepath.Walk(*inpath, processDir)
	if err != nil {
		log.Fatal(err)
	}

	buildIndexes()
}

func buildIndexes() {
	for k := range indexes {
		indexFile := filepath.Join(k, "_index.adoc")
		dirs := []string{}
		files := []string{}

		if k != "." && k != "./" {
			dirs = append(dirs, "../")
		}

		for _, f := range indexes[k] {
			if strings.HasSuffix(f, "/") && f != "./" {
				dirs = append(dirs, f)
			} else if f != "_index.adoc" && f != "./" {
				// not sure why ./ was getting in here
				files = append(files, f)
			}
		}

		// Don't want to write indexes unless there are .adocs or dirs
		if len(files) == 0 && len(dirs) == 1 {
			continue
		}

		f, err := os.OpenFile(indexFile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		t := template.Must(template.New("index").Parse(indexTpl))
		err = t.Execute(f, struct {
			DirName string
			Dirs    []string
			Files   []string
		}{k, dirs, files})
		if err != nil {
			log.Fatal(err)
		}

		info, err := os.Stat(indexFile)
		if err != nil {
			log.Fatal(err)
		}

		// Run the build command
		outFile := filepath.Join(*outPath, k, "index.html")
		buildFileForce(info, indexFile, outFile, true)
	}
}

func stripExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func buildFile(info os.FileInfo, inFile, outFile string) error {
	return buildFileForce(info, inFile, outFile, false)
}

func buildFileForce(info os.FileInfo, inFile, outFile string, force bool) error {
	buildFile, err := os.Stat(outFile)
	if err != nil || buildFile.ModTime().Before(info.ModTime()) || force {
		args := []string{"-o", outFile}
		for _, r := range requires {
			args = append(args, "-r", r)
		}
		args = append(args, inFile)
		err := exec.Command("asciidoctor", args...).Run()
		if err != nil {
			log.Printf("Err on %s: %s", inFile, err)
		}

	}
	outFile = stripExt(outFile) + ".pdf"
	buildFile, err = os.Stat(outFile)
	if (err != nil || buildFile.ModTime().Before(info.ModTime()) || force) && *pdf {
		args := []string{"-o", outFile}
		for _, r := range requires {
			args = append(args, "-r", r)
		}
		args = append(args, inFile)
		err := exec.Command("asciidoctor-pdf", args...).Run()
		if err != nil {
			log.Printf("Err on %s: %s", inFile, err)
		}
	}
	return nil
}

func processDir(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Println(err)
		return nil
	}

	// without cleaning path, it appears we sometimes put files in the . index
	// instead of the ./ index
	path = filepath.Clean(path)
	base := filepath.Dir(path)

	if strings.HasPrefix(path, *outPath) {
		return filepath.SkipDir
	}

	for _, name := range excludeNames {
		if info.Name() == name {
			return filepath.SkipDir
		}
	}

	if _, ok := indexes[path]; info.IsDir() && !ok {
		indexes[path] = make([]string, 0)
	}

	pathRelOutput := strings.TrimLeft(path, *inpath)

	if info.IsDir() {
		indexes[base] = append(indexes[base], info.Name()+"/")
	}

	for _, name := range mediaNames {
		if info.Name() == name && info.IsDir() {
			// TODO: Consider writing contents to the index
			err := os.MkdirAll(filepath.Join(*outPath, pathRelOutput), os.ModePerm)
			if err != nil {
				return err
			}
			outFile := filepath.Join(*outPath, pathRelOutput)
			cpCmd := exec.Command("cp", "-rf", path+"/", outFile)
			err = cpCmd.Run()
			if err != nil {
				log.Printf("Err: cp -rf %s %s", path+"/", outFile)
				return err
			}
			return filepath.SkipDir
		}
	}

	if filepath.Ext(info.Name()) == ".adoc" && info.Name() != "_index.adoc" {
		fileName := stripExt(info.Name())
		outFile := filepath.Join(*outPath, filepath.Dir(pathRelOutput), fileName+".html")
		indexes[base] = append(indexes[base], info.Name())

		err := buildFile(info, path, outFile)
		if err != nil {
			return err
		}
	}

	return nil
}

var indexTpl = `= Index of {{.DirName}}

{{if .Dirs}}
.Directories
{{- range $d := .Dirs}}
* link:{{$d}}[]
{{- end}}
{{end -}}

{{- if .Files}}
.Files
{{- range $f := .Files}}
* <<{{$f}}#>>
{{- end}}
{{end -}}
`
