> ~~This project has been sunset as Crunchyroll moved to a DRM-only system. See [#362](https://github.com/crunchy-labs/crunchy-cli/issues/362).~~
>
> Well there is one endpoint which still has DRM-free streams, I guess I still have a bit time until (finally) everything is DRM-only.

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

- [Scoop](https://scoop.sh/)

  For Windows users, we support the [scoop](https://scoop.sh/#/) command-line installer.

  ```shell
  $ scoop bucket add extras
  $ scoop install extras/crunchy-cli
  ```

- [Homebrew](https://brew.sh/)

  For macOS/linux users, we support the [brew](https://brew.sh/#/) command-line installer. Packages are compiled by the [homebrew project](https://formulae.brew.sh/formula/crunchy-cli), and will also install the `openssl@3` and `ffmpeg` dependencies.

  ```shell
  $ brew install crunchy-cli
  ```

  Supported archs: `x86_64_linux`, `arm64_monterey`, `sonoma`, `ventura`

- [Nix](https://nixos.org/)

  This requires [nix](https://nixos.org) and you'll probably need `--extra-experimental-features "nix-command flakes"`, depending on your configurations.

  ```shell
  $ nix <run|shell|develop> github:crunchy-labs/crunchy-cli
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
You can authenticate with your credentials (email:password) or by using a refresh token.

- <span id="global-credentials">Credentials</span>

  ```shell
  $ crunchy-cli --credentials "email:password" <command>
  ```

- <span id="global-anonymous">Stay Anonymous</span>

  Login without an account (you won't be able to access premium content):

  ```shell
  $ crunchy-cli --anonymous <command>
  ```

### Global settings

You can set specific settings which will be

- <span id="global-verbose">Verbose output</span>

  If you want to include debug information in the output, use the `-v` / `--verbose` flag to show it.

  ```shell
  $ crunchy-cli -v <command>
  ```

  This flag can't be used in combination with `-q` / `--quiet`.

- <span id="global-quiet">Quiet output</span>

  If you want to hide all output, use the `-q` / `--quiet` flag to do so.
  This is especially useful if you want to pipe the output video to an external program (like a video player).

  ```shell
  $ crunchy-cli -q <command>
  ```

  This flag can't be used in combination with `-v` / `--verbose`.

- <span id="global-lang">Language</span>

  By default, the resulting metadata like title or description are shown in your system language (if Crunchyroll supports it, else in English).
  If you want to show the results in another language, use the `--lang` flag to set it.

  ```shell
  $ crunchy-cli --lang de-DE <command>
  ```

- <span id="global-experimental-fixes">Experimental fixes</span>

  Crunchyroll constantly changes and breaks its services or just delivers incorrect answers.
  The `--experimental-fixes` flag tries to fix some of those issues.
  As the *experimental* in `--experimental-fixes` states, these fixes may or may not break other functionality.

  ```shell
  $ crunchy-cli --experimental-fixes <command>
  ```

  For an overview which parts this flag affects, see the [documentation](https://docs.rs/crunchyroll-rs/latest/crunchyroll_rs/crunchyroll/struct.CrunchyrollBuilder.html) of the underlying Crunchyroll library, all functions beginning with `stabilization_` are applied.

- <span id="global-proxy">Proxy</span>

  The `--proxy` flag supports https and socks5 proxies to route all your traffic through.
  This may be helpful to bypass the geo-restrictions Crunchyroll has on certain series.
  You are also able to set in which part of the cli a proxy should be used.
  Instead of a normal url you can also use: `<url>:` (only proxies api requests), `:<url>` (only proxies download traffic), `<url>:<url>` (proxies api requests through the first url and download traffic through the second url).

  ```shell
  $ crunchy-cli --proxy socks5://127.0.0.1:8080 <command>
  ```

  Make sure that proxy can either forward TLS requests, which is needed to bypass the (cloudflare) bot protection, or that it is configured so that the proxy can bypass the protection itself.

- <span id="global-user-agent">User Agent</span>

  There might be cases where a custom user agent is necessary, e.g. to bypass the cloudflare bot protection (#104).
  In such cases, the `--user-agent` flag can be used to set a custom user agent.

  ```shell
  $ crunchy-cli --user-agent "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 5.1; Trident/4.0)" <command>
  ```
  
  Default is the user agent, defined in the underlying [library](https://github.com/crunchy-labs/crunchyroll-rs).

- <span id="global-speed-limit">Speed limit</span>

  If you want to limit how fast requests/downloads should be, you can use the `--speed-limit` flag. Allowed units are `B` (bytes), `KB` (kilobytes) and `MB` (megabytes).

  ```shell
  $ crunchy-cli --speed-limit 10MB
  ```

### Login

The `login` command can store your session, so you don't have to authenticate every time you execute a command.

```shell
# save the refresh token which gets generated when login with credentials.
# your email and password won't be stored at any time on disk
$ crunchy-cli login --credentials "email:password"
```

With the session stored, you do not need to pass `--credentials` / `--anonymous` anymore when you want to execute a command.

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

- <span id="download-audio">Audio language</span>

  Set the audio language with the `-a` / `--audio` flag.
  This only works if the url points to a series since episode urls are language specific.

  ```shell
  $ crunchy-cli download -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is your system locale. If not supported by Crunchyroll, `en-US` (American English) is the default.

- <span id="download-subtitle">Subtitle language</span>

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.
  In formats that support it (.mp4, .mov and .mkv ), subtitles are stored as soft-subs. All other formats are hardsubbed: the subtitles will be burned into the video track (cf. [hardsub](https://www.urbandictionary.com/define.php?term=hardsub)) and thus can not be turned off.

  ```shell
  $ crunchy-cli download -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is none.

- <span id="download-output">Output template</span>

  Define an output template by using the `-o` / `--output` flag.

  ```shell
  $ crunchy-cli download -o "ditf.mp4" https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

  Default is `{title}.mp4`. See the [Template Options section](#output-template-options) below for more options.

- <span id="download-output-specials">Output template for special episodes</span>

  Define an output template which only gets used when the episode is a special (episode number is 0 or has non-zero decimal places) by using the `--output-special` flag.

  ```shell
  $ crunchy-cli download --output-specials "Special EP - {title}" https://www.crunchyroll.com/watch/GY8D975JY/veldoras-journal
  ```
  
  Default is the template, set by the `-o` / `--output` flag. See the [Template Options section](#output-template-options) below for more options.

- <span id="download-universal-output">Universal output</span>

  The output template options can be forced to get sanitized via the `--universal-output` flag to be valid across all supported operating systems (Windows has a lot of characters which aren't allowed in filenames...).

  ```shell
  $ crunchy-cli download --universal-output -o https://www.crunchyroll.com/watch/G7PU4XD48/tales-veldoras-journal-2
  ```

- <span id="download-resolution">Resolution</span>

  The resolution for videos can be set via the `-r` / `--resolution` flag.

  ```shell
  $ crunchy-cli download -r worst https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

  Default is `best`.

- <span id="download-language-tagging">Language tagging</span>

  You can force the usage of a specific language tagging in the output file with the `--language-tagging` flag.
  This might be useful as some video players doesn't recognize the language tagging Crunchyroll uses internally.

  ```shell
  $ crunchy-cli download --language-tagging ietf https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="download-ffmpeg-preset">FFmpeg Preset</span>

  You can specify specific built-in presets with the `--ffmpeg-preset` flag to convert videos to a specific coding while downloading.
  Multiple predefined presets how videos should be encoded (h264, h265, av1, ...) are available, you can see them with `crunchy-cli download --help`.
  If you need more specific ffmpeg customizations you could either convert the output file manually or use ffmpeg output arguments as value for this flag.

  ```shell
  $ crunchy-cli download --ffmpeg-preset av1-lossless https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="download-ffmpeg-threads">FFmpeg threads</span>

  If you want to manually set how many threads FFmpeg should use, you can use the `--ffmpeg-threads` flag. This does not work with every codec/preset and is skipped entirely when specifying custom ffmpeg output arguments instead of a preset for `--ffmpeg-preset`.

  ```shell
  $ crunchy-cli download --ffmpeg-threads 4 https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="download-skip-existing">Skip existing</span>

  If you re-download a series but want to skip episodes you've already downloaded, the `--skip-existing` flag skips the already existing/downloaded files.

  ```shell
  $ crunchy-cli download --skip-existing https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- <span id="download-skip-specials">Skip specials</span>

  If you doesn't want to download special episodes, use the `--skip-specials` flag to skip the download of them.

  ```shell
  $ crunchy-cli download --skip-specials https://www.crunchyroll.com/series/GYZJ43JMR/that-time-i-got-reincarnated-as-a-slime[S2]
  ```

- <span id="download-include-chapters">Include chapters</span>

  Crunchyroll sometimes provide information about skippable events like the intro or credits.
  These information can be stored as chapters in the resulting video file via the `--include-chapters` flag.

  ```shell
  $ crunchy-cli download --include-chapters https://www.crunchyroll.com/watch/G0DUND0K2/the-journeys-end
  ```

- <span id="download-yes">Yes</span>

  Sometimes different seasons have the same season number (e.g. Sword Art Online Alicization and Alicization War of Underworld are both marked as season 3), in such cases an interactive prompt is shown which needs user further user input to decide which season to download.
  The `--yes` flag suppresses this interactive prompt and just downloads all seasons.

  ```shell
  $ crunchy-cli download --yes https://www.crunchyroll.com/series/GR49G9VP6/sword-art-online
  ```

  If you've passed the `-q` / `--quiet` [global flag](#global-settings), this flag is automatically set.

- <span id="download-force-hardsub">Force hardsub</span>

  If you want to burn-in the subtitles, even if the output format/container supports soft-subs (e.g. `.mp4`), use the `--force-hardsub` flag to do so.

  ```shell
  $ crunchy-cli download --force-hardsub -s en-US https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="download-threads">Threads</span>

  To increase the download speed, video segments are downloaded simultaneously by creating multiple threads.
  If you want to manually specify how many threads to use when downloading, do this with the `-t` / `--threads` flag.

  ```shell
  $ crunchy-cli download -t 1 https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  The default thread count is the count of cpu threads your pc has.

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

- <span id="archive-audio">Audio languages</span>

  Set the audio language with the `-a` / `--audio` flag. Can be used multiple times.

  ```shell
  $ crunchy-cli archive -a ja-JP -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is your system locale (if not supported by Crunchyroll, `en-US` (American English) and `ja-JP` (Japanese) are used).

- <span id="archive-subtitle">Subtitle languages</span>

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.

  ```shell
  $ crunchy-cli archive -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is `all` subtitles.

- <span id="archive-output">Output template</span>

  Define an output template by using the `-o` / `--output` flag.
  _crunchy-cli_ exclusively uses the [`.mkv`](https://en.wikipedia.org/wiki/Matroska) container format, because of its ability to store multiple audio, video and subtitle tracks at once.

  ```shell
  $ crunchy-cli archive -o "{title}.mkv" https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is `{title}.mkv`. See the [Template Options section](#output-template-options) below for more options.

- <span id="archive-output-specials">Output template for special episodes</span>

  Define an output template which only gets used when the episode is a special (episode number is 0 or has non-zero decimal places) by using the `--output-special` flag.
  _crunchy-cli_ exclusively uses the [`.mkv`](https://en.wikipedia.org/wiki/Matroska) container format, because of its ability to store multiple audio, video and subtitle tracks at once.

  ```shell
  $ crunchy-cli archive --output-specials "Special EP - {title}" https://www.crunchyroll.com/watch/GY8D975JY/veldoras-journal
  ```

  Default is the template, set by the `-o` / `--output` flag. See the [Template Options section](#output-template-options) below for more options.

- <span id="archive-universal-output">Universal output</span>

  The output template options can be forced to get sanitized via the `--universal-output` flag to be valid across all supported operating systems (Windows has a lot of characters which aren't allowed in filenames...).

  ```shell
  $ crunchy-cli archive --universal-output -o https://www.crunchyroll.com/watch/G7PU4XD48/tales-veldoras-journal-2
  ```

- <span id="archive-resolution">Resolution</span>

  The resolution for videos can be set via the `-r` / `--resolution` flag.

  ```shell
  $ crunchy-cli archive -r worst https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is `best`.

- <span id="archive-merge">Merge behavior</span>

  Due to censorship or additional intros, some episodes have multiple lengths for different languages.
  In the best case, when multiple audio & subtitle tracks are used, there is only one *video* track and all other languages can be stored as audio-only.
  But, as said, this is not always the case.
  With the `-m` / `--merge` flag you can define the behaviour when an episodes' video tracks differ in length.
  Valid options are `audio` - store one video and all other languages as audio only; `video` - store the video + audio for every language; `auto` - detect if videos differ in length: if so, behave like `video` - otherwise like `audio`.
  Subtitles will always match the primary audio and video.

  ```shell
  $ crunchy-cli archive -m audio https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is `auto`.

- <span id="archive-merge-auto-tolerance">Merge auto tolerance</span>

  Sometimes two video tracks are downloaded with `--merge` set to `auto` even if they only differ some milliseconds in length which shouldn't be noticeable to the viewer.
  To prevent this, you can specify a range in milliseconds with the `--merge-auto-tolerance` flag that only downloads one video if the length difference is in the given range.

  ```shell
  $ crunchy-cli archive -m auto --merge-auto-tolerance 100 https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  
  Default are `200` milliseconds.

- <span id="archive-sync-start">Sync start</span>

  If you want that all videos of the same episode should start at the same time and `--merge` doesn't fit your needs (e.g. one video has an intro, all other doesn't), you might consider using the `--sync-start`.
  It tries to sync the timing of all downloaded audios to match one video.
  This is done by downloading the first few segments/frames of all video tracks that differ in length and comparing them frame by frame.
  The flag takes an optional value determines how accurate the syncing is, generally speaking everything over 15 begins to be more inaccurate and everything below 6 is too accurate (and won't succeed).
  When the syncing fails, the command is continued as if `--sync-start` wasn't provided for this episode.

  Default is `7.5`.

- <span id="archive-language-tagging">Language tagging</span>

  You can force the usage of a specific language tagging in the output file with the `--language-tagging` flag.
  This might be useful as some video players doesn't recognize the language tagging Crunchyroll uses internally.

  ```shell
  $ crunchy-cli archive --language-tagging ietf https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- <span id="archive-ffmpeg-preset">FFmpeg Preset</span>

  You can specify specific built-in presets with the `--ffmpeg-preset` flag to convert videos to a specific coding while downloading.
  Multiple predefined presets how videos should be encoded (h264, h265, av1, ...) are available, you can see them with `crunchy-cli archive --help`.
  If you need more specific ffmpeg customizations you could either convert the output file manually or use ffmpeg output arguments as value for this flag.

  ```shell
  $ crunchy-cli archive --ffmpeg-preset av1-lossless https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="archive-ffmpeg-threads">FFmpeg threads</span>

  If you want to manually set how many threads FFmpeg should use, you can use the `--ffmpeg-threads` flag. This does not work with every codec/preset and is skipped entirely when specifying custom ffmpeg output arguments instead of a preset for `--ffmpeg-preset`.

  ```shell
  $ crunchy-cli archive --ffmpeg-threads 4 https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```

- <span id="archive-default-subtitle">Default subtitle</span>

  `--default-subtitle` Set which subtitle language is to be flagged as **default** and **forced**.

  ```shell
  $ crunchy-cli archive --default-subtitle en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is none.

- <span id="archive-include-fonts">Include fonts</span>

  You can include the fonts required by subtitles directly into the output file with the `--include-fonts` flag. This will use the embedded font for subtitles instead of the system font when playing the video in a video player which supports it.

  ```shell
  $ crunchy-cli archive --include-fonts https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- <span id="archive-include-chapters">Include chapters</span>

  Crunchyroll sometimes provide information about skippable events like the intro or credits.
  These information can be stored as chapters in the resulting video file via the `--include-chapters` flag.
  This flag only works if `--merge` is set to `audio` because chapters cannot be mapped to a specific video steam.

  ```shell
  $ crunchy-cli archive --include-chapters https://www.crunchyroll.com/watch/G0DUND0K2/the-journeys-end
  ```

- <span id="archive-skip-existing">Skip existing</span>

  If you re-download a series but want to skip episodes you've already downloaded, the `--skip-existing` flag skips the already existing/downloaded files.

  ```shell
  $ crunchy-cli archive --skip-existing https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- <span id="archive-skip-existing-method">Skip existing method</span>

  By default, already existing files are determined by their name and the download of the corresponding episode is skipped.
  But sometimes Crunchyroll adds dubs or subs to an already existing episode and these changes aren't recognized and `--skip-existing` just skips it.
  This behavior can be changed by the `--skip-existing-method` flag. Valid options are `audio` and `subtitle` (if the file already exists but the audio/subtitle are less from what should be downloaded, the episode gets downloaded and the file overwritten).

  ```shell
  $ crunchy-cli archive --skip-existing-method audio --skip-existing-method video https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

- <span id="archive-skip-specials">Skip specials</span>

  If you doesn't want to download special episodes, use the `--skip-specials` flag to skip the download of them.

  ```shell
  $ crunchy-cli archive --skip-specials https://www.crunchyroll.com/series/GYZJ43JMR/that-time-i-got-reincarnated-as-a-slime[S2]
  ```

- <span id="archive-yes">Yes</span>

  Sometimes different seasons have the same season number (e.g. Sword Art Online Alicization and Alicization War of Underworld are both marked as season 3), in such cases an interactive prompt is shown which needs user further user input to decide which season to download.
  The `--yes` flag suppresses this interactive prompt and just downloads all seasons.

  ```shell
  $ crunchy-cli archive --yes https://www.crunchyroll.com/series/GR49G9VP6/sword-art-online
  ```

  If you've passed the `-q` / `--quiet` [global flag](#global-settings), this flag is automatically set.

- <span id="archive-threads">Threads</span>

  To increase the download speed, video segments are downloaded simultaneously by creating multiple threads.
  If you want to manually specify how many threads to use when downloading, do this with the `-t` / `--threads` flag.

  ```shell
  $ crunchy-cli archive -t 1 https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  
  The default thread count is the count of cpu threads your pc has.

### Search

The `search` command is a powerful tool to query the Crunchyroll library.
It behaves like the regular search on the website but is able to further process the results and return everything it can find, from the series title down to the raw stream url.
_Using this command with the `--anonymous` flag or a non-premium account may return incomplete results._

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

- <span id="search-audio">Audio</span>

  Set the audio language to search via the `--audio` flag. Can be used multiple times.

  ```shell
  $ crunchy-cli search --audio en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

  Default is your system locale.

- <span id="search-result-limit">Result limit</span>

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

- `{title}`                    → Title of the video
- `{series_name}`              → Name of the series
- `{season_name}`              → Name of the season
- `{audio}`                    → Audio language of the video
- `{width}`                    → Width of the video
- `{height}`                   → Height of the video
- `{season_number}`            → Number of the season
- `{episode_number}`           → Number of the episode
- `{relative_episode_number}`  → Number of the episode relative to its season
- `{sequence_number}`          → Like `{episode_number}` but without possible non-number characters
- `{relative_sequence_number}` → Like `{relative_episode_number}` but with support for episode 0's and .5's
- `{release_year}`             → Release year of the video
- `{release_month}`            → Release month of the video
- `{release_day} `             → Release day of the video
- `{series_id}`                → ID of the series
- `{season_id}`                → ID of the season
- `{episode_id}`               → ID of the episode

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
