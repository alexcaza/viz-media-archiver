# Viz Media Archiver

This was written for personal use as a way to archive digital goods acquired through the Viz Media app.

**This isn't feature complete, and requires an active subscription to Viz/Shonen Jump to work.**

## Required envionment variables
`MAX_ID`: The highest id to try and search for (there's no search/list endpoint, so we iterate arbitrarily through ids 1..MAX_ID)
`INSTANCE_ID:` Must be pulled through making a request via mobile APP with a MITM proxy
`DEVICE_ID:` Must be pulled through making a request via mobile APP with a MITM proxy
`USER_JWT:` Must be pulled through making a request via mobile APP with a MITM proxy
`USER_ID:` Must be pulled through making a request via mobile APP with a MITM proxy
 
## Generating the series list
Running `viz-media --generate-listing` will iterate through ids 1..MAX_ID and add all the of the found manga series to the `series_list` table in the `viz.db` sqlite3 database.

There isn't a clear search/manga list endpoint, so we have to do it the hacky way!

This will take a while. The higher your MAX_ID, the longer it'll take. This is because we use `time.Sleep` to:
1. avoid being rate limited and
2. hammering Viz's servers for no reason and being a bad actor

The functionality exists in `main.go` to regenerate this list (and could be re-run if you come across a series in the app that is not in the list). But, I'd **strongly recommend** not running it often. And if/when you do run it, you're doing it at your own risk!

## Warnings
I don't recommend others use this project. This was written as an experiment to poke around undocumented APIs and work with Golang. I have no idea how Viz/Shonen Jump would/will respond to this, and could result in an account/personal ban for anyone who uses it.
