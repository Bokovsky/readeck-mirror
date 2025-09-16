#!/usr/bin/python3

# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

import json
import os
from contextlib import chdir, contextmanager
from datetime import date
from subprocess import call, check_call, check_output
from tempfile import TemporaryDirectory
from urllib import parse, request
from urllib.error import HTTPError

SITE_CONFIG_REPO = "https://github.com/j0k3r/graby-site-config.git"


def versiontuple(v):
    return tuple(map(int, (v.split("."))))


@contextmanager
def branch(name: str):
    check_call(["git", "checkout", "-B", name])
    yield name
    check_call(["git", "checkout", "main"])


def commit_changes(files: list[str], message: str):
    check_call(["git", "add"] + files)
    rc = call(["git", "diff-index", "--quiet", "HEAD", "--"] + files)
    if rc == 0:
        return

    # fmt:off
    check_call(
        [
            "git", "commit",
            "-m", message,
            "--no-signoff", "--no-gpg-sign",
        ], env={
            **os.environ,
            "GIT_AUTHOR_NAME": "Readeck Bot",
            "GIT_AUTHOR_EMAIL": "bot@readeck.com",
            "GIT_COMMITTER_NAME": "Readeck Bot",
            "GIT_COMMITTER_EMAIL": "bot@readeck.com",
        },
    )
    # fmt:on


def push_changes(repository_url: str, branch_name: str):
    rc = call(["git", "diff-index", "--quiet", "main"])
    if rc == 0:
        return

    check_call(["git", "push", "--force", repository_url, branch_name])


def create_pr(api_url: str, api_token: str, repository: str, branch_name: str):
    r = request.Request(
        url=f"{api_url}/repos/{repository}/pulls",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"token {api_token}",
        },
        data=json.dumps(
            {
                "base": "main",
                "head": branch_name,
                "title": f"Dependencies update [{date.today()}]",
            }
        ).encode("utf-8"),
    )
    try:
        request.urlopen(r)
    except HTTPError as e:
        if e.status != 409:
            raise


def update_go_version():
    data = check_output(["go", "mod", "edit", "-json"])
    current = json.loads(data)["Go"]

    rsp = request.urlopen("https://go.dev/VERSION?m=text")
    latest = rsp.readline().decode().strip().lstrip("go")
    rsp.close()

    cv = versiontuple(current)
    lv = versiontuple(latest)

    if lv > cv and lv[1] == cv[1]:
        check_call(["go", "mod", "edit", f"-go={latest}"])


def update_go_dependencies():
    check_call(["go", "get", "-t", "-u", "-v", "./..."])
    check_call(["go", "mod", "tidy"])


def update_js_dependencies():
    with chdir("web"):
        # fmt:off
        check_call(
            [
                "npm", "exec", "-y", "--",
                "npm-check-updates",
                "-t", "minor",
                "--peer", "--upgrade",
                "--install", "always",
            ]
        )
        # fmt: on
        check_call(["npx", "-y", "update-browserslist-db@latest"])


def update_site_config_files():
    with TemporaryDirectory() as folder:
        # fmt: off
        check_call(
            [
                "git", "clone", "--depth", "1",
                SITE_CONFIG_REPO, "--single-branch",
                "--branch", "master",
                folder,
            ]
        )
        check_call(
            [
                "go", "run", "./tools/ftr",
                folder,
                "pkg/extract/contentscripts/assets/site-config",
            ], env={
                **os.environ,
                "GOWORK": "off",
            },
        )
        # fmt: on


def main():
    api_url = os.environ.get("GITHUB_API_URL")
    api_user = os.environ.get("API_USER")
    api_token = os.environ.get("API_TOKEN")
    repository = os.environ.get("GITHUB_REPOSITORY")
    repository_url = (
        check_output(["git", "remote", "get-url", "origin"]).decode("utf-8").strip()
    )

    url = parse.urlparse(repository_url)
    if url.scheme == "https":
        url = url._replace(netloc=f"{api_user}:{api_token}@{url.netloc}")
        repository_url = url.geturl()
    else:
        repository_url = None

    print(f"API_URL:    {api_url}")
    print(f"API USER:   {api_user}")
    print(f"REPOSITORY: {repository}")

    with branch("chore/updates") as branch_name:
        update_go_version()
        update_go_dependencies()
        commit_changes(
            ["go.mod", "go.sum"],
            "Updated Go dependencies",
        )

        update_js_dependencies()
        commit_changes(
            ["web/package.json", "web/package-lock.json"],
            "Updated JS dependencies",
        )

        update_site_config_files()
        commit_changes(
            ["pkg/extract/contentscripts/assets/site-config"],
            "Updated Site Config files",
        )

        rc = call(["git", "diff-index", "--quiet", "main"])
        if rc == 0:
            print("no new updates")
            return

        if repository_url:
            push_changes(repository_url, branch_name)

        if api_url and api_token and repository:
            create_pr(api_url, api_token, repository, branch_name)


if __name__ == "__main__":
    main()
