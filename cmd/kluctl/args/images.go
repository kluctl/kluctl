package args

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/spf13/cobra"
	"strings"
)

var (
	FixedImages     []string
	FixedImagesFile string
	UpdateImages    bool
)

func AddImageArgs(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&FixedImages, "fixed-image", "F", nil, "Pin an image to a given version. Expects '--fixed-image=image<:namespace:deployment:container>=result'")
	cmd.Flags().StringVar(&FixedImagesFile, "fixed-images-file", "", "Use .yml file to pin image versions. See output of list-images sub-command or read the documentation for details about the output format")
	cmd.Flags().BoolVarP(&UpdateImages, "update-images", "u", false, "This causes kluctl to prefer the latest image found in registries, based on the `latest_image` filters provided to 'images.get_image(...)' calls. Use this flag if you want to update to the latest versions/tags of all images. '-u' takes precedence over `--fixed-image/--fixed-images-file`, meaning that the latest images are used even if an older image is given via fixed images.")

	_ = cmd.MarkFlagFilename("fixed-images-file", "yml", "yaml")
}

func LoadFixedImagesFromArgs() (*types.FixedImagesConfig, error) {
	var ret types.FixedImagesConfig

	if FixedImagesFile != "" {
		err := types.LoadFixedImagesConfig(FixedImagesFile, &ret)
		if err != nil {
			return nil, err
		}
	}

	for _, fi := range FixedImages {
		e, err := buildFixedImageEntryFromArg(fi)
		if err != nil {
			return nil, err
		}
		ret.Images = append(ret.Images, *e)
	}

	return &ret, nil
}

func buildFixedImageEntryFromArg(arg string) (*types.FixedImage, error) {
	s := strings.Split(arg, "=")
	if len(s) != 2 {
		return nil, fmt.Errorf("--fixed-image expects 'image<:namespace:deployment:container>=result'")
	}
	image := s[0]
	result := s[1]

	s = strings.Split(image, ":")
	e := types.FixedImage{
		Image:       s[0],
		ResultImage: result,
	}

	if len(s) >= 2 {
		e.Namespace = &s[2]
	}
	if len(s) >= 3 {
		e.Deployment = &s[3]
	}
	if len(s) >= 4 {
		e.Container = &s[4]
	}
	if len(s) >= 5 {
		return nil, fmt.Errorf("--fixed-image expects 'image<:namespace:deployment:container>=result'")
	}

	return &e, nil
}
