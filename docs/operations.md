# Managing doctree.org

For now, doctree.org is hosted in the Sourcegraph DigitalOcean account in [the doctree project](https://cloud.digitalocean.com/projects/00778e28-7044-4d03-9ab6-72b252afe76e/resources?i=2a039a) while things are so early stages / experimental. If you need access, let us know in the #doctree Slack channel.

## Adding a repository

```sh
task ops-add-repo -- https://github.com/django/django
```

## Wipe all server data

```sh
task ops-remove-all
```

## Restore our sample repositories

```sh
task ops-add-sample-repos
```

## Deploy latest version

```sh
task ops-deploy
```
