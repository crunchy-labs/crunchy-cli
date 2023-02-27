# crunchy-cli

A pure [Rust](https://www.rust-lang.org/) CLI for [Crunchyroll](https://www.crunchyroll.com).

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
  <a href="#%EF%B8%8F-disclaimer">Disclaimer 📜</a>
  •
  <a href="#-license">License ⚖</a>
</p>

> We are in no way affiliated with, maintained, authorized, sponsored, or officially associated with Crunchyroll LLC or any of its subsidiaries or affiliates.
> The official Crunchyroll website can be found at [crunchyroll.com](https://crunchyroll.com/).

> This README belongs to the _master_ branch which is currently under heavy development towards the next major version (3.0).
> It is mostly stable but some issues may still occur.
> If you do not want to use an under-development / pre-release version, head over to the _[golang](https://github.com/crunchy-labs/crunchy-cli/tree/golang)_ branch which contains the EOL but last stable version (and documentation for it).

## ✨ Features

- Download single videos and entire series from [Crunchyroll](https://www.crunchyroll.com).
- Archive episodes or seasons in an `.mkv` file with multiple subtitles and audios.
- Specify a range of episodes to download from an anime.

## 💾 Get the executable

### 📥 Download the latest binaries

Check out the [releases](https://github.com/crunchy-labs/crunchy-cli/releases) tab and get the binary from the latest (pre-)release.

### 🛠 Build it yourself

Since we do not support every platform and architecture you may have to build the project yourself.
This requires [git](https://git-scm.com/) and [Cargo](https://doc.rust-lang.org/cargo).
```shell
$ git clone https://github.com/crunchy-labs/crunchy-cli
$ cd crunchy-cli
$ cargo build --release
$ cargo install --force --path .
```
After the binary has been built successfully it is available in `target/release`.

## 🖥️ Usage

> All shown commands are examples 🧑🏼‍🍳

crunchy-cli requires you to log in.
Though you can use a non-premium account, you will not have access to premium content without a subscription.
You can authenticate with your credentials (username:password) or by using a refresh token.

- Credentials
  - ```shell
    $ crunchy --credentials "user:password"
    ```
- Refresh Token
  - To obtain a refresh token, you have to log in at [crunchyroll.com](https://www.crunchyroll.com/) and extract the `etp_rt` cookie.
    The easiest way to get it is via a browser extension which lets you export your cookies, like [Cookie-Editor](https://cookie-editor.cgagnier.ca/) ([Firefox](https://addons.mozilla.org/en-US/firefox/addon/cookie-editor/) / [Chrome](https://chrome.google.com/webstore/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm)).
    When installed, look for the `etp_rt` entry and extract its value.
  - ```shell
    $ crunchy --etp-rt "4ebf1690-53a4-491a-a2ac-488309120f5d"
    ```
- Stay Anonymous
  - Skip the login check:
  - ```shell 
    $ crunchy --anonymous
    ```

### Login

crunchy-cli can store your session, so you don't have to authenticate every time you execute a command.

Note that the `login` keyword has to be used *last*.

```shell
$ crunchy --etp-rt "4ebf1690-53a4-491a-a2ac-488309120f5d" login
```

With the session stored, you do not need to use `--credentials` / `--etp-rt` anymore. This does not work with `--anonymous`.

### Download

**Supported urls**
- Single episode
  ```shell
  $ crunchy download https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
- Series
  ```shell
  $ crunchy download https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

**Options**
- Audio language

  Set the audio language with the `-a` / `--audio` flag.
  This only works if the url points to a series since episode urls are language specific.
  ```shell
  $ crunchy download -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system locale. If not supported by Crunchyroll, `en-US` (American English) is the default.

- Subtitle language

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.
  The subtitles will be burned into the video track (cf. [hardsub](https://www.urbandictionary.com/define.php?term=hardsub)) and thus can not be turned off.
  ```shell
  $ crunchy download -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is none.

- Output template

  Define an output template by using the `-o` / `--output` flag.
  If you want to use any other file format than [`.ts`](https://en.wikipedia.org/wiki/MPEG_transport_stream) you need [ffmpeg](https://ffmpeg.org/).
  ```shell
  $ crunchy download -o "ditf.ts" https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `{title}.ts`.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy download -r worst https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `best`.

### Archive

**Supported urls**
- Series

  Only series urls are supported, because episode urls are locked to a single audio language.
  ```shell
  $ crunchy archive https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

**Options**
- Audio languages

  Set the audio language with the `-a` / `--audio` flag. Can be used multiple times.
  ```shell
  $ crunchy archive -a ja-JP -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system locale (if not supported by Crunchyroll, `en-US` (American English) and `ja-JP` (Japanese) are used).

- Subtitle languages

  Besides the audio, you can specify the subtitle language by using the `-s` / `--subtitle` flag.
  ```shell
  $ crunchy archive -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `all` subtitles.

- Output template

  Define an output template by using the `-o` / `--output` flag.
  crunchy-cli uses the [`.mkv`](https://en.wikipedia.org/wiki/Matroska) container format, because of it's ability to store multiple audio, video and subtitle tracks at once.
  ```shell
  $ crunchy archive -o "{title}.mkv" https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `{title}.mkv`.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy archive -r worst https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
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
  $ crunchy archive -m audio https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `auto`.

- Default subtitle

  `--default_subtitle` Set which subtitle language is to be flagged as **default** and **forced**.
  ```shell
  $ crunchy archive --default_subtitle en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is none.

- Subtitle optimizations

  Crunchyroll's subtitles look weird in some players (#66).
  This can be fixed by adding a specific entry to the subtitles.
  Even though this entry is a de facto standard, it is not defined in the official specification for the `.ass` format (cf. [Advanced SubStation Subtitles](https://wiki.videolan.org/SubStation_Alpha)). This could cause compatibility issues, but no issues have been reported yet.
  `--no_subtitle_optimizations` disables these optimizations.
  ```shell
  $ crunchy archive --no_subtitle_optimizations https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

### Episode filtering

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

In practice, it would look like this: `https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx[E1-E5]`

# 📜 Disclaimer

This tool is **ONLY** meant for private use. You will need a subscription to [`💳 Crunchyroll Premium 💳`](https://www.crunchyroll.com/welcome#plans) to download premium content.

**You are entirely responsible for what happens to files you downloaded through crunchy-cli.**

# ⚖ License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0) - see the [LICENSE](LICENSE) file for more details.
