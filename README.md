# demucs-web

Provides a simple WebUI for using yt-dlp and demucs.

Data: `/wd/data/demucs-web.sqlite`, `/wd/data/results`, (`/wd/data/temp`)

Exposed: `:8080`

Optional environment:

|name|default|description|
|----|-------|-----------|
|`HEAD_HTML`|``|raw HTML displayed at top of index page|
|`WORKERS`|`1`|how many jobs to run concurrently|
|`TIMEOUT`|`1h`|maximum job run time|
|`JOBS_HTDEMUCS_FT`|`8`|tuning for named model|
|`JOBS_HTDEMUCS`|`8`|tuning for named model|
|`JOBS_HTDEMUCS_6S`|`8`|tuning for named model|
|`JOBS_HDEMUCS_MMI`|`16`|tuning for named model|
|`JOBS_MDX_EXTRA`|`16`|tuning for named model|
|`JOBS_NOGRACEFULRETRY`|`0`|integer boolean, disables retries being reduced to 1 demucs job|

Tuning: While using 1 job is magnitudes slower, using too many (definition unclear) results demucs failing with various weird errors. Retries always use 1 job, unless `JOBS_NOGRACEFULRETRY=1`.
