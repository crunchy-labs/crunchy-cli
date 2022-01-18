**This branch is under highly development, so it may contain errors which are making compiling not possible**

# crunchyroll-go

A [Go](https://golang.org) library & cli for the undocumented [crunchyroll](https://www.crunchyroll.com) api.

**You surely need a crunchyroll premium account to get full (api) access.**

<p align="center">
  <a href="https://github.com/ByteDream/crunchyroll-go">
    <img src="https://img.shields.io/github/languages/code-size/ByteDream/crunchyroll-go?style=flat-square" alt="Code size">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/releases/latest">
    <img src="https://img.shields.io/github/downloads/ByteDream/crunchyroll-go/total?style=flat-square" alt="Download Badge">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/ByteDream/crunchyroll-go?style=flat-square" alt="License">
  </a>
  <a href="https://golang.org">
    <img src="https://img.shields.io/github/go-mod/go-version/ByteDream/crunchyroll-go?style=flat-square" alt="Go version">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/releases/latest">
    <img src="https://img.shields.io/github/v/release/ByteDream/crunchyroll-go?style=flat-square" alt="Release">
  </a>
  <a href="https://discord.gg/gUWwekeNNg">
    <img src="https://img.shields.io/discord/915659846836162561?label=discord&style=flat-square" alt="Discord">
  </a>
</p>

<p align="center">
  <a href="#%EF%B8%8F-cli">CLI 🖥️</a>
  •
  <a href="#-library">Library 📚</a>
  •
  <a href="#-credits">Credits 🙏</a>
  •
  <a href="#️-notice">Notice 🗒️</a>
  •
  <a href="#-license">License ⚖</a>
</p>

## 🖥️ CLI

#### ✨ Features
- Download single videos and entire series from [crunchyroll](https://www.crunchyroll.com)

#### Get the executable
- 📥 Download the latest binaries [here](https://github.com/ByteDream/crunchyroll-go/releases/latest) or get it from below
  - [Linux (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_linux)
  - [Windows (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_windows.exe)
  - [MacOS (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_darwin)
- If you use Arch btw. or any other Linux distro which is based on Arch Linux, you can download the package via the [AUR](https://aur.archlinux.org/packages/crunchyroll-go/):
  ```
  $ yay -S crunchyroll-go
  ```
- 🛠 Build it yourself
    - use `make` (requires `go` to be installed):
  ```
  $ git clone https://github.com/ByteDream/crunchyroll-go
  $ cd crunchyroll-go
  $ make && sudo make install
  ```
    - use `go`:
  ```
  $ git clone https://github.com/ByteDream/crunchyroll-go
  $ cd crunchyroll-go/cmd/crunchyroll-go
  $ go build -o crunchy
  ```

### 📝 Examples

#### Login
Before you can do something, you have to login first.

This can be performed via crunchyroll account email and password.
```
$ crunchy login user@example.com password
```

or via session id
```
$ crunchy login --session-id 8e9gs135defhga790dvrf2i0eris8gts
```

#### Download

**With the cli you can download single videos or entire series.**

By default the cli tries to download the episode with your system language as audio.
If no streams with your system language are available, the video will be downloaded with japanese audio and hardsubbed subtitles in your system language.
**If your system language is not supported, an error message will be displayed and en-US (american english) will be chosen as language.**

```
$ crunchy download https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

With `-r best` the video(s) will have the best available resolution (mostly 1920x1080 / Full HD).

```
$ crunchy download -r best https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

The file is by default saved as a `.ts` (mpeg transport stream) file.
`.ts` files may can't be played or are looking very weird (it depends on the video player you are using).
With the `-o` flag, you can change the name (and file ending) of the output file.
So if you want to save it as, for example, `mp4` file, just name it `whatever.mp4`.
**You need [ffmpeg](https://ffmpeg.org) to store the video in other file formats.**

```
$ crunchy download -o "daaaaaaaaaaaaaaaarling.ts" https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

With the `--audio` flag you can specify which audio the video should have and with `--subtitle` which subtitle it should have.
Type `crunchy help download` to see all available locales.

```
$ crunchy download --audio ja-JP --subtitle de-DE https://www.crunchyroll.com/darling-in-the-franxx
```

##### Flags
- `--audio` » forces audio of the video(s)
- `--subtitle` » forces subtitle of the video(s)
- `--no-hardsub` » forces that the subtitles are stored as a separate file and are not directly embedded into the video
- `--only-sub` » downloads only the subtitles without the corresponding video

- `-d`, `--directory` » directory to download the video(s) to
- `-o`, `--output` » name of the output file

- `-r`, `--resolution` » the resolution of the video(s). `best` for best resolution, `worst` for worst

- `--alternative-progress` » shows an alternative, not so user-friendly progress instead of the progress bar

- `-g`, `--goroutines` » sets how many parallel segment downloads should be used

#### Help
- General help
  ```
  $ crunchy help
  ```
- Login help
  ```
  $ crunchy help login
  ```
- Download help
  ```
  $ crunchy help download
  ```

#### Global flags
These flags you can use across every sub-command

- `-q`, `--quiet` » disables all output
- `-v`, `--verbose` » shows additional debug output
- `--color` » adds color to the output (works only on not windows systems)

- `-p`, `--proxy` » use a proxy to hide your ip / redirect your traffic

- `-l`, `--locale` » the language to display video specific things like the title. default is your system language

## 📚 Library
Download the library via `go get`

```
$ go get github.com/ByteDream/crunchyroll-go
```

### 📝 Examples
```go
func main() {
    // login with credentials 
    crunchy, err := crunchyroll.LoginWithCredentials("user@example.com", "password", crunchyroll.US, http.DefaultClient)
    if err != nil {
        panic(err)
    }

    // finds a series or movie by a crunchyroll link
    video, err := crunchy.FindVideo("https://www.crunchyroll.com/darling-in-the-franxx")
    if err != nil {
        panic(err)
    }

    series := video.(*crunchyroll.Series)
    seasons, err := series.Seasons()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d seasons for series %s\n", len(seasons), series.Title)

    // search `Darling` and return 20 results
    series, movies, err := crunchy.Search("Darling", 20)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d series and %d movies for query `Darling`\n", len(series), len(movies))
}
```

```go
func main() {
    crunchy, err := crunchyroll.LoginWithSessionID("8e9gs135defhga790dvrf2i0eris8gts", crunchyroll.US, http.DefaultClient)
    if err != nil {
        panic(err)
    }

    // returns an episode slice with all episodes which are matching the given url.
    // the episodes in the returning slice differs from the underlying streams, but are all pointing to the first ditf episode
    episodes, err := crunchy.FindEpisode("https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d episodes\n", len(episodes))
}
```

<h4 align="center">Structure</h4>

Because of the apis structure, it can lead very fast much redundant code for simple tasks, like getting all episodes
with japanese audio and german subtitle. For this case and some other, the api has a utility called `Structure` in its utils.

```go
func main() {
    crunchy, err := crunchyroll.LoginWithCredentials("user@example.com", "password", crunchyroll.US, http.DefaultClient)
    if err != nil {
        panic(err)
    }

    // search `Darling` and return 20 results (series and movies) or less
    series, movies, err := crunchy.Search("Darling", 20)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d series and %d movies for search query `Darling`\n", len(series), len(movies))

    seasons, err := series[0].Seasons()
    if err != nil {
        panic(err)
    }

    // in the crunchyroll.utils package, you find some structs which can be used to simplify tasks.
    // you can recursively search all underlying content
    seriesStructure := utils.NewSeasonStructure(seasons)

    // this returns every format of all the above given seasons
    formats, err := seriesStructure.Formats()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d formats\n", len(formats))

    filteredFormats, err := seriesStructure.FilterFormatsByLocales(crunchyroll.JP, crunchyroll.DE, true)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Found %d formats with japanese audio and hardsubbed german subtitles\n", len(filteredFormats))

    // reverse sorts the formats after their resolution by calling a sort type which is also defined in the api utils
    // and stores the format with the highest resolution in a variable
    sort.Sort(sort.Reverse(utils.FormatsByResolution(filteredFormats)))
    format := formats[0]
    // get the episode from which the format is a child
    episode, err := seriesStructure.FilterEpisodeByFormat(format)
    if err != nil {
        panic(err)
    }

    file, err := os.Create(fmt.Sprintf("%s.ts", episode.Title))
    if err != nil {
        panic(err)
    }

    // download the format to the file
    if err := format.DownloadGoroutines(file, 4, nil); err != nil {
        panic(err)
    }
    fmt.Printf("Downloaded %s with %s resolution and %.2f fps as %s\n", episode.Title, format.Video.Resolution, format.Video.FPS, file.Name())

    // for more useful structure function just let your IDE's autocomplete make its thing
}
```

As you can see in the example above, most of the `crunchyroll.utils` Structure functions are returning errors. There is
a build-in functionality with are avoiding causing the most errors and let you safely ignore them as well.
**Note that errors still can appear**

```go
func main() {
    crunchy, err := crunchyroll.LoginWithCredentials("user@example.com", "password", crunchyroll.US, http.DefaultClient)
    if err != nil {
        panic(err)
    }

    foundEpisodes, err := crunchy.FindEpisode("https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575")
    if err != nil {
    	panic(err)
    }
    episodeStructure := utils.NewEpisodeStructure(foundEpisodes)

    // this function recursively calls all api endpoints, receives everything and stores it in memory,
    // so that after executing this, no more request to the crunchyroll server has to be made.
    // note that it could cause much network load while running this method.
    //
    // you should check the InitAllState before, because InitAll could have been already called or
    // another function has the initialization as side effect and re-initializing everything
    // will change every pointer in the struct which can cause massive problems afterwards. 
    if !episodeStructure.InitAllState() {
        if err := episodeStructure.InitAll(); err != nil {
            panic(err)
        }
    }

    formats, _ := episodeStructure.Formats()
    streams, _ := episodeStructure.Streams()
    episodes, _ := episodeStructure.Episodes()
    fmt.Printf("Initialized %d formats, %d streams and %d episodes\n", len(formats), len(streams), len(episodes))
}
```

### Tests
You can also run test to see if the api works correctly.
Before doing this, make sure to either set your crunchyroll email and password or sessions as environment variable.
The email variable has to be named `EMAIL` and the password variable `PASSWORD`. If you want to use your session id, the variable must be named `SESSION_ID`.

You can run the test via `make`
```
$ make test
```

or via `go` directly
```
$ go test .
```

# 🙏 Credits

### [Kamyroll-Python](https://github.com/hyugogirubato/Kamyroll-Python)
- Extracted all api endpoints and the login process from this

### [m3u8](https://github.com/oopsguy/m3u8)
- Decrypting mpeg stream files

### All libraries
- [m3u8](https://github.com/grafov/m3u8) (not the m3u8 library from above) » mpeg stream info library
- [cobra](https://github.com/spf13/cobra) » cli library

# 🗒️ Notice

Sometimes the download stops without a reason on linux and does not go further. In this case the `tmpfs` / `/tmp` directory may be full. Execute `df /tmp` to see how much of the space is used.

I would really appreciate if someone rewrites the complete cli. I'm not satisfied with it's current structure but at the moment I have no time and no desire to do it myself.

# ⚖ License

This project is licensed under the GNU Lesser General Public License v3.0 (LGPL-3.0) - see the [LICENSE](LICENSE) file
for more details.
