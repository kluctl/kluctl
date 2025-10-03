# Contributing

As all Open Source projects, Kluctl lives from contributions. If you consider contributing, we are welcoming you to do so.

## Communication

Contributing to Open Source projects also means that you are communicating with other members of the community. Please
follow the [Code of Conduct](./CODE_OF_CONDUCT.md) whenever you communicate.

## Creating issues / Reporting bugs

Probably the easiest way to contribute is to create issues on GitHub. An issue could for example be a bug report or
a feature request. It's also fine to create an issue that requests some change, for example to documentation. Issues
can also be used as reminders/TODOs that something needs to be done in the future.

The project is still in early stage when it comes to project management, so please bear with us if processes are still
a bit "loose".

One thing we'd like to ask for is to first search for issues that might already represent what you plan to report.
In many cases it turns out that other people stumbled across the same thing already. If not, then you're "lucky" to
report it first. :slightly_smiling_face:

It might also be useful to ask inside the #kluctl channel of the [CNCF Slack](https://slack.cncf.io) if you are unsure
about your issue.

## Finding issues to work on

Check the [open issues](https://github.com/kluctl/kluctl/issues) and see if you can find something that you believe you
could help with.

## Contributing/Modifying documentation

The next level of contribution is modifying documentation. Fork the repository and start working on the changes locally.
When done, commit the changes, push them and create a pull request. If your pull request fixes an issue, add the proper
`Fixes: #xxx` line so that the issue can be auto-closed.

Please note that we follow [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) when committing.

## Contributing/Modifying source code

To fix a bug or add a feature, follow the same procedure as described in the previous chapter. Read the
[DEVELOPMENT](./DEVELOPMENT.md) guidelines on how to build and run Kluctl from source.

Additionally, you should ensure that your changes don't break anything. You can do this by running the
[test suite](./DEVELOPMENT.md#how-to-run-the-test-suite) and by testing your changes manually. Adding new tests for
fixed bugs and/or new features is also nice to see. Reviewers will also point out when tests would be of value.

However, don't worry if you feel overwhelmed (e.g. with tests). You can also create a `[WIP]` pull request and ask for
help.

Please note that we follow [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) when committing.

## Conventional commits

As mentioned before, we follow [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) when committing.
These commits are then used to create release notes and properly bump version numbers.

We might reconsider this approach in the future, especially when we re-design the
[release process](./DEVELOPMENT.md#releasing-process).
