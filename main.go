package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/Arvintian/NCMconverter/converter"
	"github.com/Arvintian/NCMconverter/ncm"
	"github.com/Arvintian/NCMconverter/path"
	"github.com/Arvintian/NCMconverter/tag"
	"github.com/urfave/cli/v2"
	"github.com/xxjwxc/gowp/workpool"
)

var version = "1.0.0"
var cmd = struct {
	output string
	tag    bool
	thread int
}{}
var pool *workpool.WorkPool

func main() {
	app := &cli.App{
		Name:  "NCMConverter",
		Usage: "convert ncm file to mp3 or flac format",
		Authors: []*cli.Author{
			{Name: "Arvin", Email: "arvintian8@gmail.com"},
		},
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Value:       ``,
				Usage:       "output dir",
				Destination: &cmd.output,
			},
			&cli.BoolFlag{
				Name:        "tag",
				Aliases:     []string{"t"},
				Value:       true,
				Usage:       "if adding tag to music file",
				Destination: &cmd.tag,
			},
			&cli.IntFlag{
				Name:        "thread",
				Aliases:     []string{"n"},
				Value:       10,
				Usage:       "max thread count",
				Destination: &cmd.thread,
			},
		},
		Commands: []*cli.Command{
			{
				Name: "version",
				Action: func(c *cli.Context) error {
					cli.ShowVersion(c)
					return nil
				},
			},
		},
		Action: action,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func action(c *cli.Context) error {
	if c.NArg() < 1 {
		cli.ShowAppHelpAndExit(c, 1)
	}

	args := c.Args().Slice()

	cmd.output = path.Clean(cmd.output)
	pool = workpool.New(cmd.thread)

	res, err := resolvePath(args)
	if err != nil {
		log.Printf("resolving file path failed: %v", err)
	}

	for _, pt := range res {
		p := pt
		pool.Do(func() error {
			err := convert(p, cmd.output)
			if err != nil {
				log.Printf("Convert %v failed: %v", p, err)
			} else {
				log.Printf("Convert %s success", p)
			}
			return nil
		})
	}
	pool.Wait()
	return nil
}

func resolvePath(args []string) ([]string, error) {
	var res []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			log.Printf("Can not find file: %s", arg)
			continue
		}
		if info.IsDir() {
			res, err = findNCMInDir(arg)
			if err != nil {
				log.Printf("find ncm file in %s failed: %v", arg, err)
			}
		} else {
			res = append(res, arg)
		}
	}
	return res, nil
}

func convert(filePath, dir string) error {
	nf, err := ncm.NewNcmFile(filePath)
	if err != nil {
		return err
	}

	defer nf.Close()

	err = nf.Parse()
	if err != nil {
		return err
	}

	cv := converter.NewConverter(nf)
	err = cv.HandleAll()
	if err != nil {
		return err
	}

	if dir == "" {
		dir = nf.FileDir
	}

	fileName := strings.Split(cv.FileName, cv.Ext)
	dir = path.Join(dir, fileName[0]+"."+cv.MetaData.Format)

	err = writeToFile(dir, cv.MusicData)
	if err != nil {
		return err
	}

	if cmd.tag {
		Tag(dir, cv.Cover.Detail, cv.MetaData)
	}
	return nil
}

func writeToFile(dst string, data []byte) error {
	info, err := os.Stat(path.Dir(dst))
	if err != nil && info == nil {
		os.MkdirAll(path.Dir(dst), 0644)
	}
	fd, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer fd.Close()

	_, err = fd.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func Tag(path string, imageData []byte, meta *converter.Meta) error {
	tg, err := tag.NewTagger(path, meta.Format)
	if err != nil {
		return err
	}
	err = tag.TagAudioFileFromMeta(tg, imageData, meta)
	return err
}

func findNCMInDir(dir string) ([]string, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0)
	for _, info := range infos {
		if info.IsDir() {
			tmp, _ := findNCMInDir(path.Join(dir, info.Name()))
			if tmp != nil {
				res = append(res, tmp...)
			}
		} else {
			if strings.HasSuffix(info.Name(), ".ncm") {
				res = append(res, path.Join(dir, info.Name()))
			}
		}
	}
	return res, nil
}
