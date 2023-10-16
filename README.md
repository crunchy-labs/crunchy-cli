# crunchy-cli

👇 A Command-line downloader for [Crunchyroll](https://www.crunchyroll.com).

<p align="center">
  <a href="https://github.com/crunchy-labs/crunchy-cli">
    <img src="https://img.shields.io/github/languages/code-size/crunchy-labs/crunchy-cli?style=flat-square" alt="Code size">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/releases/latest">
    <img src="https://img.shields.io/github/downloads/crunchy-labs/crunchy-cli/total?style=flat-square" alt="Download Badge">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/crunchy-labs/crunchy-cli?style=flat-square" alt="License">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/releases">
    <img src="https://img.shields.io/github/v/release/crunchy-labs/crunchy-cli?include_prereleases&style=flat-square" alt="Release">
  </a>
  <a href="https://discord.gg/PXGPGpQxgk">
    <img src="https://img.shields.io/discord/994882878125121596?label=discord&style=flat-square" alt="Discord">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/actions/workflows/ci.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/crunchy-labs/crunchy-cli/ci.yml?branch=master&style=flat-square" alt="CI">
  </a>
</p>

<p align="center">
  <a href="#%EF%B8%8F-usage">Usage 🖥️</a>
  •
  <a href="#-disclaimer">Disclaimer 📜</a>
  •
  <a href="#-license">License ⚖</a>
</p>

> We are in no way affiliated with, maintained, authorized, sponsored, or officially associated with Crunchyroll LLC or any of its subsidiaries or affiliates.
> The official Crunchyroll website can be found at [www.crunchyroll.com](https://www.crunchyroll.com/).

## ✨ Features

- Download single videos and entire series from [Crunchyroll](https://www.crunchyroll.com).
- Archive episodes or seasons in an `.mkv` file with multiple subtitles and audios.
- Specify a range of episodes to download from an anime.
- Search through the Crunchyroll collection and return metadata (title, duration, direct stream link, ...) of all media types.

## 💾 Get the executable

### 📥 Download the latest binaries

Check out the [releases](https://github.com/crunchy-labs/crunchy-cli/releases) tab and get the binary from the latest (pre-)release.

### 📦 Get it via a package manager

- [AUR](https://aur.archlinux.org/)

  If you're using Arch or an Arch based Linux distribution you are able to install our [AUR](https://aur.archlinux.org/) package.
  You need an [AUR helper](https://wiki.archlinux.org/title/AUR_helpers) like [yay](https://github.com/Jguer/yay) to install it.
  ```shell
  # this package builds crunchy-cli manually (recommended)
  $ yay -S crunchy-cli
  # this package installs the latest pre-compiled release binary
  $ yay -S crunchy-cli-bin
  ```

- [Nix](https://nixos.org/)

  This requires [nix](https://nixos.org) and you'll probably need `--extra-experimental-features "nix-command flakes"`, depending on your configurations.
  ```shell
  $ nix <run|shell|develop> github:crunchy-labs/crunchy-cli
  ```

- [Scoop](https://scoop.sh/)

  For Windows users, we support the [scoop](https://scoop.sh/#/) command-line installer.
  ```shell
  $ scoop bucket add extras
  $ scoop install extras/crunchy-cli
  ```

### 🛠 Build it yourself

Since we do not support every platform and architecture you may have to build the project yourself.
This requires [git](https://git-scm.com/) and [Cargo](https://doc.rust-lang.org/cargo).
```shell
$ git clone https://github.com/crunchy-labs/crunchy-cli
$ cd crunchy-cli
# either just build it (will be available in ./target/release/crunchy-cli)...
$ cargo build --release
# ... or install it globally
$ cargo install --force --path .
```

## 🖥️ Usage

> All shown commands are examples 🧑🏼‍🍳

### Global Flags

crunchy-cli requires you to log in.
Though you can use a non-premium account, you will not have access to premium content without a subscription.
You can authenticate with your credentials (user:password) or by using a refresh token.

- Credentials

  ```shell
  $ crunchy-cli --credentials "user:password" <command>
  ```
- Refresh Token

  To obtain a refresh token, you have to log in at [crunchyroll.com](https://www.crunchyroll.com/) and extract the `etp_rt` cookie.
  The easiest way to get it is via a browser extension which lets you export your cookies, like [Cookie-Editor](https://cookie-editor.cgagnier.ca/) ([Firefox](https://addons.mozilla.org/en-US/firefox/addon/cookie-editor/) / [Chrome](https://chrome.google.com/webstore/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm)).
  When installed, look for the `etp_rt` entry and extract its value.
  ```shell
  $ crunchy-cli --etp-rt "4ebf1690-53a4-491a-a2ac-488309120f5d" <command>
  ```
- Stay Anonymous

  Login without an account (you won't be able to access premium content):
  ```shell 
  $ crunchy-cli --anonymous <command>
  ```

### Global settings

You can set specific settings which will be

- Verbose output

  If you want to include debug information in the output, use the `-v` / `--verbose` flag to show it.
  ```shell
  $ crunchy-cli -v <command>
  ```
  This flag can't be used with `-q` / `--quiet`.

- Quiet output

  If you want to hide all output, use the `-q` / `--quiet` flag to do so.
  This is especially useful if you want to pipe the output video to an external program (like a video player).
  ```shell
  $ crunchy-cli -q <command>
  ```

- Language

  By default, the resulting metadata like title or description are shown in your system language (if Crunchyroll supports it, else in English).
  If you want to show the results in another language, use the `--lang` flag to set it.
  ```shell
  $ crunchy-cli --lang de-DE <command>
  ```

- Experimental fixes

  Crunchyroll constantly changes and breaks its services or just delivers incorrect answers.
  The `--experimental-fixes` flag tries to fix some of those issues.
  As the *experimental* in `--experimental-fixes` states, these fixes may or may not break other functionality.
  ```shell
  $ crunchy-cli --experimental-fixes <command>
  ```
  For an overview which parts this flag affects, see the [documentation](https://docs.rs/crunchyroll-rs/latest/crunchyroll_rs/crunchyroll/struct.CrunchyrollBuilder.html) of the underlying Crunchyroll library, all functions beginning with `stabilization_` are applied.

- Proxy

  The `--proxy` flag supports https and socks5 proxies to route all your traffic through.
  This may be helpful to bypass the geo-restrictions Crunchyroll has on certain series.
  ```shell
  $ crunchy-cli --proxy socks5://127.0.0.1:8080 <command>
  ```
  Make sure that proxy can either forward TLS requests, which is needed to bypass the (cloudflare) bot protection, or that it is configured so that the proxy can bypass the protection itself.

### Login

The `login` command can store your session, so you don't have to authenticate every time you execute a command.

```shell
# save the refresh token which gets generated when login with credentials.
# your username/email and password won't be stored at any time on disk
$ crunchy-cli login --credentials "user:password"
# save etp-rt cookie
$ crunchy-cli login --etp-rt "4ebf1690-53a4-491a-a2ac-488309120f5d"
```

With the session stored, you do not need to pass `--credentials` / `--etp-rt` / `--anonymous` anymore when you want to execute a command.

### Download

The `download` command lets you download episodes with a specific audio language and optional subtitles.

**Supported urls**
- Single episode (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli download https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
- Series (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli download https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

**Options**
- Audio language

  Set the audio language with the `-a` / `--audio` flag.
  This only works if the url points to a series since episode urls are language specific.
  ```shell
  $ crunchy-cli download -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system locale. If not supported by Crunchyroll, `en-US` (American English) is the default.

- Subtitle language

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.
  In formats that support it (.mp4, .mov and .mkv ), subtitles are stored as soft-subs. All other formats are hardsubbed: the subtitles will be burned into the video track (cf. [hardsub](https://www.urbandictionary.com/define.php?term=hardsub)) and thus can not be turned off.
  ```shell
  $ crunchy-cli download -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is none.

- Output template

  Define an output template by using the `-o` / `--output` flag.
  ```shell
  $ crunchy-cli download -o "ditf.mp4" https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `{title}.mp4`. See the [Template Options section](#output-template-options) below for more options.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy-cli download -r worst https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `best`.

- FFmpeg Preset

  You can specify specific built-in presets with the `--ffmpeg-preset` flag to convert videos to a specific coding while downloading.
  Multiple predefined presets how videos should be encoded (h264, h265, av1, ...) are available, you can see them with `crunchy-cli download --help`.
  If you need more specific ffmpeg customizations you could either convert the output file manually or use ffmpeg output arguments as value for this flag.
  ```shell
  $ crunchy-cli downlaod --ffmpeg-preset av1-lossless https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- Skip existing

  If you re-download a series but want to skip episodes you've already downloaded, the `--skip-existing` flag skips the already existing/downloaded files.
  ```shell
  $ crunchy-cli download --skip-existing https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- Yes

  Sometimes different seasons have the same season number (e.g. Sword Art Online Alicization and Alicization War of Underworld are both marked as season 3), in such cases an interactive prompt is shown which needs user further user input to decide which season to download.
  The `--yes` flag suppresses this interactive prompt and just downloads all seasons.
  ```shell
  $ crunchy-cli download --yes https://www.crunchyroll.com/series/GR49G9VP6/sword-art-online
  ```
  If you've passed the `-q` / `--quiet` [global flag](#global-settings), this flag is automatically set.

- Force hardsub

  If you want to burn-in the subtitles, even if the output format/container supports soft-subs (e.g. `.mp4`), use the `--force-hardsub` flag to do so.
  ```shell
  $ crunchy-cli download --force-hardsub -s en-US https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

### Archive

The `archive` command lets you download episodes with multiple audios and subtitles and merges it into a `.mkv` file.

**Supported urls**
- Single episode (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli archive https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
- Series (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli archive https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

**Options**
- Audio languages

  Set the audio language with the `-a` / `--audio` flag. Can be used multiple times.
  ```shell
  $ crunchy-cli archive -a ja-JP -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system locale (if not supported by Crunchyroll, `en-US` (American English) and `ja-JP` (Japanese) are used).

- Subtitle languages

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.
  ```shell
  $ crunchy-cli archive -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `all` subtitles.

- Output template

  Define an output template by using the `-o` / `--output` flag.
  crunchy-cli uses the [`.mkv`](https://en.wikipedia.org/wiki/Matroska) container format, because of it's ability to store multiple audio, video and subtitle tracks at once.
  ```shell
  $ crunchy-cli archive -o "{title}.mkv" https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `{title}.mkv`. See the [Template Options section](#output-template-options) below for more options.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy-cli archive -r worst https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `best`.

- Merge behavior

  Due to censorship, some episodes have multiple lengths for different languages.
  In the best case, when multiple audio & subtitle tracks are used, there is only one *video* track and all other languages can be stored as audio-only.
  But, as said, this is not always the case.
  With the `-m` / `--merge` flag you can define the behaviour when an episodes' video tracks differ in length.
  Valid options are `audio` - store one video and all other languages as audio only; `video` - store the video + audio for every language; `auto` - detect if videos differ in length: if so, behave like `video` - otherwise like `audio`.
  Subtitles will always match the primary audio and video.
  ```shell
  $ crunchy-cli archive -m audio https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `auto`.

- FFmpeg Preset

  You can specify specific built-in presets with the `--ffmpeg-preset` flag to convert videos to a specific coding while downloading.
  Multiple predefined presets how videos should be encoded (h264, h265, av1, ...) are available, you can see them with `crunchy-cli archive --help`.
  If you need more specific ffmpeg customizations you could either convert the output file manually or use ffmpeg output arguments as value for this flag.
  ```shell
  $ crunchy-cli archive --ffmpeg-preset av1-lossless https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- Default subtitle

  `--default-subtitle` Set which subtitle language is to be flagged as **default** and **forced**.
  ```shell
  $ crunchy-cli archive --default-subtitle en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is none.

- Skip existing

  If you re-download a series but want to skip episodes you've already downloaded, the `--skip-existing` flag skips the already existing/downloaded files.
  ```shell
  $ crunchy-cli archive --skip-existing https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- Yes

  Sometimes different seasons have the same season number (e.g. Sword Art Online Alicization and Alicization War of Underworld are both marked as season 3), in such cases an interactive prompt is shown which needs user further user input to decide which season to download.
  The `--yes` flag suppresses this interactive prompt and just downloads all seasons.
  ```shell
  $ crunchy-cli archive --yes https://www.crunchyroll.com/series/GR49G9VP6/sword-art-online
  ```
  If you've passed the `-q` / `--quiet` [global flag](#global-settings), this flag is automatically set.

### Search

**Supported urls/input**
- Single episode (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli search https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
- Series (with [episode filtering](#episode-filtering))
  ```shell
  $ crunchy-cli search https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
- Search input
  ```shell
  $ crunchy-cli search "darling in the franxx"
  ```

**Options**
- Audio

  Set the audio language to search via the `--audio` flag. Can be used multiple times.
  ```shell
  $ crunchy-cli search --audio en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system locale.

- Result limit

  If your input is a search term instead of an url, you have multiple options to control which results to process.
  The `--search-top-results-limit` flag sets the limit of top search results to process.
  `--search-series-limit` sets the limit of only series, `--search-movie-listing-limit` of only movie listings, `--search-episode-limit` of only episodes and `--search-music-limit` of only concerts and music videos.
  ```shell
  $ crunchy-cli search --search-top-results-limit 10 "darling in the franxx"
  # only return series which have 'darling' in it. do not return top results which might also be non-series items
  $ crunchy-cli search --search-top-results-limit 0 --search-series-limit 10 "darling"
  # this returns 2 top results, 3 movie listings, 5 episodes and 1 music item as result
  $ crunchy-cli search --search-top-results-limit 2 --search-movie-listing-limit 3 --search-episode-limit 5 --search-music-limit 1 "test"
  ```
  Default is `5` for `--search-top-results-limit`, `0` for all others.

- Output template

  The search command is designed to show only the specific information you want.
  This is done with the `-o`/`--output` flag.
  You can specify keywords in a specific pattern, and they will get replaced in the output text.
  The required pattern for this begins with `{{`, then the keyword, and closes with `}}` (e.g. `{{episode.title}}`).
  For example, if you want to get the title of an episode, you can use `Title: {{episode.title}}` and `{{episode.title}}` will be replaced with the episode title.
  You can see all supported keywords with `crunchy-cli search --help`.
  ```shell
  $ crunchy-cli search -o "{{series.title}}" https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `S{{season.number}}E{{episode.number}} - {{episode.title}}`.

---

#### Output Template Options

You can use various template options to change how the filename is processed. The following tags are available:

- `{title}`                   → Title of the video
- `{series_name}`             → Name of the series
- `{season_name}`             → Name of the season
- `{audio}`                   → Audio language of the video
- `{resolution}`              → Resolution of the video
- `{season_number}`           → Number of the season
- `{episode_number}`          → Number of the episode
- `{relative_episode_number}` → Number of the episode relative to its season
- `{series_id}`               → ID of the series
- `{season_id}`               → ID of the season
- `{episode_id}`              → ID of the episode

Example:
```shell
$ crunchy-cli archive -o "[S{season_number}E{episode_number}] {title}.mkv" https://www.crunchyroll.com/series/G8DHV7W21/dragon-ball
# Output file: '[S01E01] Secret of the Dragon Ball.mkv'
```

#### Episode filtering

Filters patterns can be used to download a specific range of episodes from a single series.

A filter pattern may consist of either a season, an episode, or a combination of the two.
When used in combination, seasons `S` must be defined before episodes `E`.

There are many possible patterns, for example:
- `...[E5]` - Download the fifth episode.
- `...[S1]` - Download the whole first season.
- `...[-S2]` - Download the first two seasons.
- `...[S3E4-]` - Download everything from season three, episode four, onwards.
- `...[S1E4-S3]` - Download season one, starting at episode four, then download season two and three.
- `...[S3,S5]` - Download season three and five.
- `...[S1-S3,S4E2-S4E6]` - Download season one to three, then episodes two to six from season four.

In practice, it would look like this:
```
https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx[E1-E5]
```

# 📜 Disclaimer

This tool is meant for private use only.
You need a [Crunchyroll Premium](https://www.crunchyroll.com/welcome#plans) subscription to access premium content.

**You are entirely responsible for what happens when you use crunchy-cli.**

# ⚖ License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for more details.
