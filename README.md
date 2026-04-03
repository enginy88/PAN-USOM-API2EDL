# PAN-USOM-API2EDL

A Go program to fetch the USOM (TR-CERT) OSINT Feed via JSON API, persist it to a SQLite database, and convert it to multiple PAN-OS EDL-compatible plain-text files — categorized by IOC type, severity, time window, and count cap.

## Brief:

This program's existence is due to the following reasons:

* PAN-OS can only read EDLs from plain TXT files. It does not support other formats like JSON, XML, RSS, STIX, or TAXII. It is sensitive to data format and only accepts one value per line.
* PAN-OS has no regex capability for EDLs to parse different types. It cannot segregate different types like IP, domain, and URL from a single EDL source. Each type needs to be defined separately.
* PAN-OS has capacity limits for EDLs. Higher end platforms have a 250k URL limit, and lower end platforms have a 100k URL limit. Other limits exist for IPs and domains depending on the platform.
* USOM does not provide separate feeds for IP, domain, and URL IOC types, unlike other CERTs. They only provide a single source with a mixture of each IOC type.
* USOM feed contains IOCs with multiple and inconsistent syntaxes for each type. Some sort of normalization is needed before use.
* USOM feed contains a huge number of IOCs. The total count exceeds PAN-OS capacity limits even for the highest platforms. As old records are never removed, the feed contains ancient and questionable entries.
* Minemald could be used to address some of these downsides formerly, but it is archived and no longer developed by PAN. Also, it produces another link in the security toolchain, which needs to be learned and maintained.

To overcome these challenges, PAN-USOM-API2EDL needs to be used as a middleware. This program:

* Fetches the full USOM IOC Feed via the paginated JSON API.
* Persists all fetched records into an in-memory SQLite database.
* Detects changes by comparing the in-memory database against the previously saved on-disk snapshot.
* Creates a rotating backup of the on-disk database when changes are detected.
* Identifies, extracts, and normalizes IP, URL, and domain IOCs using regex.
* Filters IOCs by criticality (severity) level.
* Supports time-window filtering (e.g. last 30 days, 1 year) and count-capping (e.g. top 100k most recent entries).
* Deduplicates IOCs after normalization.
* Generates many categorized EDL files in plain-text format ready for direct consumption by PAN-OS.
* Supports optional aggregated EDL modes where IOC extraction crosses USOM-assigned type boundaries.
* Generates lists sequentially or concurrently using a configurable worker pool.

## Usage:

Pre-compiled binaries can be downloaded directly from the latest release (Link here: [Latest Release](https://github.com/enginy88/PAN-USOM-API2EDL/releases/latest)). These binaries can readily be used on the systems for which they were compiled. Neither re-compiling any source code nor installing Go is needed. In case there is no pre-compiled binary presented for your system, you can refer to the [Compilation](#compilation) section.

This program only requires the `PAN-USOM-API2EDL.env` file to determine which settings it will run with. The supplied `PAN-USOM-API2EDL.env` is a sample/template — rename or copy it to `PAN-USOM-API2EDL.env` to use it. Even if the program can run with its default settings without any options set in the env file, the env file must be present and accessible.

By default, the program searches for the `PAN-USOM-API2EDL.env` file in the working directory. The working directory can be changed by passing the `-dir [PATH]` argument. (`PATH` value for `-dir` can be absolute or relative.) The working directory also determines where the generated EDL files will be placed. If you need to change the output directory without changing the working directory, you can use the `-out [PATH]` argument for that purpose. (`PATH` value for `-out` can be absolute or relative. If a relative `PATH` value for `-out` is used together with the `-dir` option, it is based on the former working directory.) These options can be explored by passing the `-usage` argument to the program.

There are multiple settings controlled in environment variable format in the `PAN-USOM-API2EDL.env` file. These settings are explained in the [Settings](#settings) section.

How to schedule this program and how to serve generated EDL files are not within the scope of this program or this documentation. Nevertheless, some hints are shared in the [Hints](#hints) section.

## Output Files:

The program generates EDL plain-text files named using the following convention:

```
edl-<type>-<severity>-<window>.txt
```

| Component | Values |
|-----------|--------|
| `<type>` | `ip`, `url`, `domain`, `mix`, `aggr_url`, `aggr_domain` |
| `<severity>` | `any` (all criticality levels), `high` (minimum criticality threshold set via `MIN_CRITICALITY`) |
| `<window>` | `30d`, `90d`, `180d`, `1y`, `3y`, `5y` (time windows), `50k`, `100k`, `150k`, `250k`, `500k`, `1m` (count caps), or `all` (no limit) |

Examples: `edl-ip-any-30d.txt`, `edl-domain-high-100k.txt`, `edl-aggr_url-any-all.txt`, `edl-mix-high-1y.txt`

**List categories and their default state:**

| Category | Description | Default |
|----------|-------------|---------|
| **Standalone** | Separate IP, URL, and domain lists with time-window and (for domains) count-capped variants | Enabled |
| **Aggregated** | Cross-type extraction: `aggr_url` (URLs from all records), `aggr_domain` (domains from URL+domain records); time-window and count-capped variants | Enabled |
| **Mix** | All IOC types combined into a single list; time-window and count-capped variants | Disabled |

Each generated file includes a header with the last update timestamp and the record count.

## Settings:

All setting options are provided with the sample `PAN-USOM-API2EDL.env` file, along with short descriptions and default values for each. Note that all lines are commented-out in the sample. To use any option, simply remove the comment token (`#`) and set the preferred value. When ready, rename or copy the file to `PAN-USOM-API2EDL.env`.

Settings can also be provided as actual environment variables, which take precedence over the env file.

```shell
# Global Settings:
API2EDL_GLOBAL__API_PATH={Enter URL of USOM API endpoint, Default: https://www.usom.gov.tr/api/address/index}
API2EDL_GLOBAL__DB_PATH={Enter path to SQLite database file, Default: usom.db}
API2EDL_GLOBAL__READ_FROM_FILE={Enter either TRUE or FALSE to read from file instead of API, Default: FALSE}
API2EDL_GLOBAL__ENABLE_CONCURRENCY={Enter either TRUE or FALSE to enable concurrent list generation, Default: FALSE}
API2EDL_GLOBAL__NUM_OF_WORKER={Enter number of concurrent workers, Default: 4}

# Log Settings:
API2EDL_LOG__VERBOSE={Enter either TRUE or FALSE to enable verbose logging, Default: FALSE}
API2EDL_LOG__WRITE_TO_DIR={Enter directory path to write log files, Default: (empty, logs to stdout)}
API2EDL_LOG__FILENAME_SUFFIX={Enter suffix string to append to log filenames, Default: (empty)}

# Request Settings:
API2EDL_REQUEST__TOTAL_TIMEOUT={Enter total operation timeout in seconds, Default: 180}
API2EDL_REQUEST__REQUEST_TIMEOUT={Enter per-request timeout in seconds, Default: 30}
API2EDL_REQUEST__ADD_RETRY_COUNT={Enter number of additional retry attempts after first request, Default: 2}
API2EDL_REQUEST__RETRY_WAIT_TIME={Enter wait time between retries in milliseconds, Default: 1000}
API2EDL_REQUEST__RETRY_MAX_WAIT_TIME={Enter maximum wait time between retries in milliseconds, Default: 5000}
API2EDL_REQUEST__ALLOW_REDIRECT={Enter either TRUE or FALSE to allow HTTP redirects, Default: FALSE}
API2EDL_REQUEST__MAX_REDIRECT={Enter maximum number of redirects to follow, Default: 2}
API2EDL_REQUEST__RESPONSE_BODY_LIMIT={Enter maximum response body size in bytes, Default: 30000000}
API2EDL_REQUEST__USER_AGENT={Enter custom User-Agent header string, Default: Mozilla/5.0 (compatible; Linux x86_64; IDEUS/1.0)}

# List Settings:
API2EDL_LIST__MIN_CRITICALITY={Enter minimum criticality level to include entries (1-5), Default: 5}
API2EDL_LIST__CREATE_STANDALONE_LISTS={Enter either TRUE or FALSE to create standalone EDL lists, Default: TRUE}
API2EDL_LIST__CREATE_AGGREGATED_LISTS={Enter either TRUE or FALSE to create aggregated EDL lists, Default: TRUE}
API2EDL_LIST__CREATE_MIX_LISTS={Enter either TRUE or FALSE to create mixed EDL lists, Default: FALSE}
API2EDL_LIST__SKIP_IF_DB_IDENTICAL={Enter either TRUE or FALSE to skip output if DB is unchanged, Default: FALSE}
```

<details>

<summary>Long explanation of each settings option: (Expand to view)</summary>

### Explanation of Settings:

**API2EDL_GLOBAL__API_PATH**

TYPE: `String` DEFAULT VALUE: `https://www.usom.gov.tr/api/address/index`

The URL of the USOM JSON API endpoint. The program fetches all paginated pages from this URL. Normally, there is no need to change this from the default value. However, it is implemented for a possible future scenario where USOM changes the URL and a quick reconfiguration is needed without recompiling the source code.

**API2EDL_GLOBAL__DB_PATH**

TYPE: `String` DEFAULT VALUE: `usom.db`

Path to the on-disk SQLite database file. After each successful fetch, the in-memory database is compared with this file. If differences are detected, the old file is renamed to `usom.db_old` as a rotating backup and a new snapshot is written. On the next run the freshest snapshot can be used directly via `READ_FROM_FILE`.

**API2EDL_GLOBAL__READ_FROM_FILE**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

When set to `TRUE`, the program skips the live API fetch and loads records directly from the on-disk SQLite database at `DB_PATH`. Useful for regenerating EDL files from an existing snapshot without making any network requests (e.g. for testing or re-generating lists after changing filter settings).

**API2EDL_GLOBAL__ENABLE_CONCURRENCY**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

When set to `TRUE`, EDL list generation tasks are dispatched to a worker pool instead of being processed one by one. For large numbers of output files this can significantly reduce total run time. The number of parallel workers is controlled by `NUM_OF_WORKER`.

**API2EDL_GLOBAL__NUM_OF_WORKER**

TYPE: `Integer` DEFAULT VALUE: `4`

The number of goroutine workers in the pool when `ENABLE_CONCURRENCY` is `TRUE`. Has no effect when concurrency is disabled.

**API2EDL_LOG__VERBOSE**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

This program has 4 levels of log output: always, error, warning, and info. When set to `FALSE`, info-level logs are suppressed. Always-level and error-level logs are never suppressed. Error-level logs indicate an unrecoverable failure that causes the program to stop.

**API2EDL_LOG__WRITE_TO_DIR**

TYPE: `String` DEFAULT VALUE: `(empty)`

Directory path where log files should be written. When empty, all output goes to stdout.

**API2EDL_LOG__FILENAME_SUFFIX**

TYPE: `String` DEFAULT VALUE: `(empty)`

Optional suffix appended to log filenames when `WRITE_TO_DIR` is set.

**API2EDL_REQUEST__TOTAL_TIMEOUT**

TYPE: `Integer` DEFAULT VALUE: `180`

Total operation timeout for the entire paginated fetch session, in seconds.

**API2EDL_REQUEST__REQUEST_TIMEOUT**

TYPE: `Integer` DEFAULT VALUE: `30`

Per-request timeout for each individual API call, in seconds.

**API2EDL_REQUEST__ADD_RETRY_COUNT**

TYPE: `Integer` DEFAULT VALUE: `2`

Number of additional retry attempts after the first failed request. A value of `2` means up to 3 total attempts per page.

**API2EDL_REQUEST__RETRY_WAIT_TIME**

TYPE: `Integer` DEFAULT VALUE: `1000`

Wait time between retry attempts, in milliseconds.

**API2EDL_REQUEST__RETRY_MAX_WAIT_TIME**

TYPE: `Integer` DEFAULT VALUE: `5000`

Maximum wait time between retries (used for exponential backoff), in milliseconds.

**API2EDL_REQUEST__ALLOW_REDIRECT**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

Whether to follow HTTP redirects. Disabled by default as a security measure; an unexpected redirect could indicate a MITM attempt against your threat feed.

**API2EDL_REQUEST__MAX_REDIRECT**

TYPE: `Integer` DEFAULT VALUE: `2`

Maximum number of redirects to follow when `ALLOW_REDIRECT` is `TRUE`.

**API2EDL_REQUEST__RESPONSE_BODY_LIMIT**

TYPE: `Integer` DEFAULT VALUE: `30000000`

Maximum response body size in bytes (default ~30 MB). Requests exceeding this limit will be rejected. Adjust if the USOM API response ever grows beyond this size.

**API2EDL_REQUEST__USER_AGENT**

TYPE: `String` DEFAULT VALUE: `Mozilla/5.0 (compatible; Linux x86_64; IDEUS/1.0)`

The User-Agent header sent with each API request.

**API2EDL_LIST__MIN_CRITICALITY**

TYPE: `Integer` DEFAULT VALUE: `5`

The minimum criticality level threshold for `high` severity lists (1 = lowest, 5 = highest). Only records with a criticality level equal to or greater than this value are included in lists with `high` severity. Records in `any` severity lists are never filtered by criticality. USOM assigns criticality from 1 to 5.

**API2EDL_LIST__CREATE_STANDALONE_LISTS**

TYPE: `Boolean` DEFAULT VALUE: `TRUE`

When `TRUE`, generates separate EDL files for each IOC type (`ip`, `url`, `domain`) across all time windows and severity levels. For the `domain` type, count-capped variants (50k to 1m) are also generated.

**API2EDL_LIST__CREATE_AGGREGATED_LISTS**

TYPE: `Boolean` DEFAULT VALUE: `TRUE`

When `TRUE`, generates aggregated EDL files that cross USOM-assigned type boundaries using forced regex extraction:

* `aggr_url` — URLs extracted from IP, URL, and domain-typed records.
* `aggr_domain` — Domains extracted from URL-typed and domain-typed records.

Time-window and count-capped variants are generated for all aggregated types.

**API2EDL_LIST__CREATE_MIX_LISTS**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

When `TRUE`, generates mixed EDL files containing all IOC types (`ip`, `url`, `domain`) combined into a single list. Time-window and count-capped variants are generated.

**API2EDL_LIST__SKIP_IF_DB_IDENTICAL**

TYPE: `Boolean` DEFAULT VALUE: `FALSE`

When `TRUE`, the program skips all EDL file generation if the freshly fetched data is identical to the previously saved on-disk database snapshot. Useful when running on a frequent schedule to avoid unnecessary disk I/O and downstream EDL refresh triggers on PAN-OS when the feed has not changed.

</details>

## Hints:

When using this program under Unix-like OSes like Linux or macOS, Cron can be used to schedule periodic execution of the program. Here is an example of a Crontab entry:

```shell
# /etc/crontab
# To run the program every hour:
0 * * * * /path_to_binary/PAN-USOM-API2EDL -dir /path_to_env_file/ -out /path_for_edl_files/
```

Under Windows OSes, the Task Scheduler tool can be used for the same purpose.

For serving the generated EDL files, Apache, Nginx, or IIS can be used to handle incoming HTTP(S) requests. For short-term testing purposes, the `http.server` module of Python 3 can be used. Here is an example of how to run it:

```python
# Serves content of the current working directory:
python3 -m http.server 8080
```

## Compilation:

If none of the pre-compiled binaries covers your environment, you can compile from source. Here are the instructions:

```shell
git clone https://github.com/enginy88/PAN-USOM-API2EDL.git
cd PAN-USOM-API2EDL
go mod tidy
make local  # Compile for your own environment.
make        # Cross-compile for all pre-selected environments.
```

Cross-compiled binaries are placed under the `bin/` directory with platform suffixes (e.g. `PAN-USOM-API2EDL_lin-amd64`, `PAN-USOM-API2EDL_mac-arm64`, `PAN-USOM-API2EDL_win-amd64.exe`). All binaries are built with `CGO_ENABLED=0` and are therefore fully self-contained — no shared libraries or runtime dependencies are required on the target system.

**NOTE:** To compile from source, Go must be installed in the environment. However, it is not necessary to run the compiled binaries. Please consult the Go website for installation instructions: [Installing Go](https://go.dev/doc/install)

## Why Golang?

Because it (cross) compiles into machine code! You can directly run ready-to-go binaries on Windows, Linux, and macOS. No installation, no libraries, no dependencies, no prerequisites... Unlike Bash/PowerShell/Python it is not interpreted at runtime, which drastically reduces runtime overhead compared to scripting languages. The decision to use a compiled language makes it run lightning fast with lower memory usage. Also, due to the statically typed nature of the Go language, it is more error-proof against possible bugs/typos.
