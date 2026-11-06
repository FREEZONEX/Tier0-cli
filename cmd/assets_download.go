package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	errDownloadRedirectLimit   = errors.New("too many download redirects")
	errDownloadRedirectInvalid = errors.New("invalid download redirect")
	errDownloadDowngrade       = errors.New("refusing HTTPS downgrade in download redirect")
	errDownloadInsecureOrigin  = errors.New("refusing insecure cross-origin download redirect")
	errDownloadGatewayRedirect = errors.New("gateway download must return a 302 redirect")
)

// Inspect the directory entry without following symbolic links. On Windows we
// cannot preserve an existing file's ACL using chmod and rename.
func downloadDestination(path, goos string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("output must be a regular file, not a symlink or special file")
	}
	if goos == "windows" {
		return nil, fmt.Errorf("replacing an existing output file is not supported on Windows")
	}
	return info, nil
}

// downloadRedirect is separate from the API policy: storage URLs can have a
// different HTTPS origin, but must never receive credentials or a Referer.
func downloadRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 5 {
		return errDownloadRedirectLimit
	}
	if len(via) == 0 || req.URL.User != nil || (req.URL.Scheme != "https" && req.URL.Scheme != "http") {
		return errDownloadRedirectInvalid
	}
	previous := via[len(via)-1].URL
	if strings.EqualFold(previous.Scheme, "https") && !strings.EqualFold(req.URL.Scheme, "https") {
		return errDownloadDowngrade
	}
	// net/http may populate Referer with the previous URL, including a signature.
	req.Header.Del("Referer")
	origin := func(u *url.URL) string { return u.Scheme + "://" + u.Host }
	crossed := !config.SameOrigin(origin(via[0].URL), origin(req.URL))
	for _, prior := range via[1:] {
		if !config.SameOrigin(origin(via[0].URL), origin(prior.URL)) {
			crossed = true
		}
	}
	if crossed {
		if !strings.EqualFold(req.URL.Scheme, "https") {
			return errDownloadInsecureOrigin
		}
		// Go can copy initial headers again at every hop; clear all of them,
		// including Referer, rather than maintaining a list of sensitive names.
		req.Header = make(http.Header)
	}
	return nil
}

var assetsDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a file from Tier0 object storage",
	Long: `Download a file from Tier0 object storage by filePath.

Examples:
  tier0 assets download --file-path workspace/.../report.csv -o ./report.csv
  tier0 assets download --file-path workspace/.../report.csv`,
	RunE: runAssetsDownload,
}

func init() {
	assetsDownloadCmd.Flags().Bool("cloud", false, "Use Cloud v2 file download endpoint only")
	assetsDownloadCmd.Flags().String("file-path", "", "File path returned by upload")
	assetsDownloadCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	assetsDownloadCmd.Flags().String("content-disposition", "", "Custom Content-Disposition")
	_ = assetsDownloadCmd.MarkFlagRequired("file-path")
}

func runAssetsDownload(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	filePath, _ := cmd.Flags().GetString("file-path")
	output, _ := cmd.Flags().GetString("output")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")
	cloud, _ := cmd.Flags().GetBool("cloud")

	profile, err := config.LoadProfile()
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if profile.APIKey == "" {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("API Key not found"), jsonMode)
	}

	query := "?filePath=" + url.QueryEscape(filePath)
	if contentDisposition != "" {
		query += "&responseContentDisposition=" + url.QueryEscape(contentDisposition)
	}
	redirected := false
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		redirected = true
		if len(via) == 1 && (req.Response == nil || req.Response.StatusCode != http.StatusFound) {
			return errDownloadGatewayRedirect
		}
		return downloadRedirect(req, via)
	}}
	get := func(version string) (*http.Response, error) {
		u := strings.TrimRight(profile.BaseURL, "/") + "/openapi/" + version + "/assets/files/download" + query
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", profile.APIKey)
		if debug {
			fmt.Fprintf(os.Stderr, "[debug] GET %s\n", u)
		}
		return client.Do(req)
	}
	resp, err := get("v2")
	if !cloud && err == nil && !redirected && resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		resp, err = get("v1")
	}
	if err != nil {
		// net/http errors can contain a complete presigned Location URL.
		for _, safe := range []error{errDownloadRedirectLimit, errDownloadRedirectInvalid, errDownloadDowngrade, errDownloadInsecureOrigin, errDownloadGatewayRedirect} {
			if errors.Is(err, safe) {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download: %w", safe), jsonMode)
			}
		}
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download request failed (network or redirect error)"), jsonMode)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download failed: status=%d", resp.StatusCode), jsonMode)
	}
	if !redirected {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("download failed: gateway did not return a 302 redirect"), jsonMode)
	}

	var out io.Writer = cmd.OutOrStdout()
	var temp *os.File
	var existing os.FileInfo
	if output != "" {
		existing, err = downloadDestination(output, runtime.GOOS)
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("output: %w", err), jsonMode)
		}
		f, err := os.CreateTemp(filepath.Dir(output), ".tier0-download-*")
		if err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("create output: %w", err), jsonMode)
		}
		temp = f
		defer func() { f.Close(); os.Remove(f.Name()) }()
		out = f
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("write output: %w", err), jsonMode)
	}
	if temp != nil {
		if existing != nil {
			if err := temp.Chmod(existing.Mode().Perm()); err != nil {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("set output permissions: %w", err), jsonMode)
			}
		}
		if err := temp.Close(); err != nil {
			return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("close output: %w", err), jsonMode)
		}
		if existing == nil {
			// Link creates the destination only if it does not already exist.
			if err := os.Link(temp.Name(), output); err != nil {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("create output: %w", err), jsonMode)
			}
		} else {
			current, err := os.Lstat(output)
			if err != nil || !current.Mode().IsRegular() || !os.SameFile(existing, current) {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("output changed during download"), jsonMode)
			}
			if err := os.Rename(temp.Name(), output); err != nil {
				return cmdutil.HandleCommandError(cmd.ErrOrStderr(), fmt.Errorf("replace output: %w", err), jsonMode)
			}
		}
	}

	if output != "" && !jsonMode {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Downloaded to %s\n", output)
	} else if jsonMode {
		fmt.Fprintln(cmd.OutOrStdout(), cmdutil.JSONString(map[string]any{"output": output, "filePath": filePath}))
	}
	return nil
}
