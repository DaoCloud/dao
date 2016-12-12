package image

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type historyOptions struct {
	image string

	human   bool
	quiet   bool
	noTrunc bool
}

// NewHistoryCommand create a new `docker history` command
func NewHistoryCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts historyOptions

	cmd := &cobra.Command{
		Use:   "history [OPTIONS] IMAGE",
		Short: "显示一个镜像的历史信息",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			return runHistory(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.human, "human", "H", true, "在人工可读的格式下打印镜像的大小和日期")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "仅显示数字ID")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "不截断命令输出内容")

	return cmd
}

func runHistory(dockerCli *client.DockerCli, opts historyOptions) error {
	ctx := context.Background()

	history, err := dockerCli.Client().ImageHistory(ctx, opts.image)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)

	if opts.quiet {
		for _, entry := range history {
			if opts.noTrunc {
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

	fmt.Fprintln(w, "镜像\t创建时间\t创建者\t大小\t备注")
	for _, entry := range history {
		imageID = entry.ID
		createdBy = strings.Replace(entry.CreatedBy, "\t", " ", -1)
		if opts.noTrunc == false {
			createdBy = stringutils.Truncate(createdBy, 45)
			imageID = stringid.TruncateID(entry.ID)
		}

		if opts.human {
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
