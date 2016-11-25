package image

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	registrytypes "github.com/docker/engine-api/types/registry"
	"github.com/spf13/cobra"
)

type searchOptions struct {
	term    string
	noTrunc bool
	limit   int
	filter  []string

	// Deprecated
	stars     uint
	automated bool
}

// NewSearchCommand create a new `docker search` command
func NewSearchCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts searchOptions

	cmd := &cobra.Command{
		Use:   "search [OPTIONS] TERM",
		Short: "在 Docker Hub(Docker官方镜像仓库)中搜索镜像",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.term = args[0]
			return runSearch(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "不截断命令输出内容")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "基于指定条件过滤命令输出内容")
	flags.IntVar(&opts.limit, "limit", registry.DefaultSearchLimit, "搜索结果的最大数")

	flags.BoolVar(&opts.automated, "automated", false, "只显示自动化构建出来的镜像")
	flags.UintVarP(&opts.stars, "stars", "s", 0, "只显示至少有 x 个用户星星的镜像")

	flags.MarkDeprecated("automated", "使用 --filter=automated=true ")
	flags.MarkDeprecated("stars", "使用 --filter=stars=3 ")

	return cmd
}

func runSearch(dockerCli *client.DockerCli, opts searchOptions) error {
	indexInfo, err := registry.ParseSearchIndexInfo(opts.term)
	if err != nil {
		return err
	}

	ctx := context.Background()

	authConfig := dockerCli.ResolveAuthConfig(ctx, indexInfo)
	requestPrivilege := dockerCli.RegistryAuthenticationPrivilegedFunc(indexInfo, "search")

	encodedAuth, err := client.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	searchFilters := filters.NewArgs()
	for _, f := range opts.filter {
		var err error
		searchFilters, err = filters.ParseFlag(f, searchFilters)
		if err != nil {
			return err
		}
	}

	options := types.ImageSearchOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
		Filters:       searchFilters,
		Limit:         opts.limit,
	}

	clnt := dockerCli.Client()

	unorderedResults, err := clnt.ImageSearch(ctx, opts.term, options)
	if err != nil {
		return err
	}

	results := searchResultsByStars(unorderedResults)
	sort.Sort(results)

	w := tabwriter.NewWriter(dockerCli.Out(), 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "名称\t描述\t星星数\t是否官方\t是否自动构建\n")
	for _, res := range results {
		// --automated and -s, --stars are deprecated since Docker 1.12
		if (opts.automated && !res.IsAutomated) || (int(opts.stars) > res.StarCount) {
			continue
		}
		desc := strings.Replace(res.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !opts.noTrunc && len(desc) > 45 {
			desc = stringutils.Truncate(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", res.Name, desc, res.StarCount)
		if res.IsOfficial {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if res.IsAutomated {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}

// SearchResultsByStars sorts search results in descending order by number of stars.
type searchResultsByStars []registrytypes.SearchResult

func (r searchResultsByStars) Len() int           { return len(r) }
func (r searchResultsByStars) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r searchResultsByStars) Less(i, j int) bool { return r[j].StarCount < r[i].StarCount }
