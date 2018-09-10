# gx-update-tracker

gx-update-tracker is a tool to help you keep track of where you are
during a complex gx-update that involves lots of decencies.  It follows
the unix philology of do one thing and do it well.  In particular it
doesn't do any work itself but keeps track of the state to aid in the
process.  It is expected that other tools can be written on top of
this.

Its advantage of just using `gx-workstation` is it gives you complete
control over the process while handling all the book keeping for you
so you don't get lost.  It is especially useful when a dependency
involves breaking changes that need to be fixed manually, or when a
dependency is a work-in-progress and you need to iterate several times
before getting it right.

Experimental, I wrote it to suit my own needs.  The interface may
change at any time.

## Install

```
go install github.com/kevina/gx-update-helper
```

## Usage

To check the list of dependencies that will need to be updated to
update `go-cid` in the current package (`go-ipfs`) use:
```
$ gx-update-helper preview go-cid
```

To start the upgrade initialize the process
```
$ gx-update-helper init go-cid
export GX_UPDATE_STATE=/home/joeuser/gocode/src/github.com/ipfs/go-ipfs/.gx-update-state.json
$ export GX_UPDATE_STATE=/home/joeuser/gocode/src/github.com/ipfs/go-ipfs/.gx-update-state.json
```

To get an overview use:
```
$ gx-update-helper status
```

To go to the first dependency the needs to be updated:
```
$ cd `gx-update-helper list ready -f '$dir' | head`
```

You should not be in `/home/joeuser/gocode/src/github.com/ipfs/go-cid`.

As this is the first package it has no dependencies so make the
required changes and run
```
$ gx release minor
$ gx-update-helper published
```

Now go to the next dependency
```
$ cd `gx-update-helper list ready -f '$dir' | head`
```

This time there are dependencies to update, no problem.  `gx-update-helper` can help:
```
$ gx-update-helper deps to-update -f 'gx update $hash' | sh
```

Now compile and test.  It is recommended you do a separate commit so
that it doesn't become part of the automatic commit created by 
`gx release`. Now do the release and again run `gx-update-helper published`.  
Repeat the last couple of steps for each dep. until you are back up to `go-ipfs`.

If something goes wrong and you have to make a change to a dependency
no problem.  Just update and republish then run `gx-update-helper
published` again.  Any already published packages that depended on the
one republished will become invalidated (you can see this by running
`gx-update-helper status`).  You can again go to each dependency
as before so you can fix them.

When it comes time to push the commits the `gx-update-helper meta`
commands can help by keeping track of the p.r.

Here is a very simple script to do the push using hub and also create
meaningful commit messages:
```
set -e
git push
echo <<EOF | hub pull-request -F - | xargs gx-update-helper meta set pr
gx update to new version of go-cid

Depends on:
`gx-update-helper deps direct -f '- \[ \] $pr'`
EOF
gx-update-helper meta get pr
```

The `xargs gx-update-helper meta set pr` part captures the output of
`hub`, which is the pull request url, and stores the value as part of
the meta-data for the current package.  The `gx-update-helper deps
direct -f '- \[ \] $pr'` lists the p.r. of the (direct) deps by
reading the metadata. And finally `gx-update-helper meta get pr`
simply displays the p.r. for reference.

Finally when your all done and ready to pin you can use
```
$ gx-update-helper to-pin
```

To list all the pins.  You can customize the output using the `-f`
option to say, create commands for the pinbot.

For additional documentation use `gx-update-helper --help` to list
available command and `gx-update-helper <cmd> --help` for detailed
documenation on that particular command.

## License

MIT © Kevin Atkinson
