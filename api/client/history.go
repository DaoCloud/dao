package client

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/go-units"
)

// CmdHistory shows the history of an image.
//
// Usage: docker history [OPTIONS] IMAGE
func (cli *DockerCli) CmdHistory(args ...string) error {
	cmd := Cli.Subcmd("history", []string{"IMAGE"}, Cli.DockerCommands["history"].Description, true)
	human := cmd.Bool([]string{"H", "-human"}, true, "在人工可读的格式下打印镜像的大小和日期")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "仅显示数字ID")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "不截断命令输出内容")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	history, err := cli.client.ImageHistory(cmd.Arg(0))
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)

	if *quiet {
		for _, entry := range history {
			if *noTrunc {
				fmt.Fprintf(w, "%s\n", entry.ID)
			} else {
				fmt.Fprintf(w, "%s\n", stringid.TruncateID(entry.ID))
			}
		}
		w.Flush()
		return nil
	}

	var imageID string
	var createdBy string
	var created string
	var size string

	fmt.Fprintln(w, "镜像\t创建世界\t创建者\t大小\t备注")
	for _, entry := range history {
		imageID = entry.ID
		createdBy = strings.Replace(entry.CreatedBy, "\t", " ", -1)
		if *noTrunc == false {
			createdBy = stringutils.Truncate(createdBy, 45)
			imageID = stringid.TruncateID(entry.ID)
		}

		if *human {
			created = units.HumanDuration(time.Now().UTC().Sub(time.Unix(entry.Created, 0))) + " ago"
			size = units.HumanSize(float64(entry.Size))
		} else {
			created = time.Unix(entry.Created, 0).Format(time.RFC3339)
			size = strconv.FormatInt(entry.Size, 10)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", imageID, created, createdBy, size, entry.Comment)
	}
	w.Flush()
	return nil
}
