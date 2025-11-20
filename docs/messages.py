# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "babel",
# ]
# ///

# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

import os
import sys
from argparse import ArgumentParser
from operator import itemgetter
from pathlib import Path

from babel.messages.catalog import Catalog
from babel.messages.extract import extract_from_file
from babel.messages.pofile import read_po, write_po

# Percentage of translated content under which a translation won't be loaded.
COMPLETION_CUTOFF = 0.90

HERE = Path(__file__).parent
ROOT = HERE / "src"

CATALOG_HEADER = """\
# Translations template for PROJECT.
# SPDX-FileCopyrightText: © YEAR Readeck <translate@readeck.com>
#
# SPDX-License-Identifier: AGPL-3.0-only
#"""

CATALOG_OPTIONS = {
    "header_comment": CATALOG_HEADER,
    "project": "Readeck User Documentation",
    "version": "1.0.0",
    "copyright_holder": "Readeck",
    "msgid_bugs_address": "translate@readeck.com",
    "last_translator": "Readeck <translate@readeck.com>",
    "language_team": "Readeck <translate@readeck.com>",
}


def extract_blocks(fileobj, keywords, comment_tags, options):
    token = None
    messages = []
    for lineno, text in enumerate(fileobj):
        lineno = lineno + 1

        if token is None:
            token = [lineno, "", [], []]
            messages = []

        if text.strip() != b"":
            messages.append(text.decode("utf-8").rstrip())
        else:
            if len(messages) > 0:
                token[2] = "\n".join(messages)
                yield token
            token = None
            messages = []

    if token is not None and len(messages) > 0:
        token[2] = "\n".join(messages)
        yield token


def po2text(catalog: Catalog, destdir: Path):
    os.makedirs(destdir, exist_ok=True)
    files = {}

    for m in catalog._messages.values():
        for x in m.locations:
            name = Path(x[0]).name
            files.setdefault(name, [])

            msg = m.string
            if m.fuzzy or msg.strip() == "":
                msg = m.id
            files[name].append((x[1], msg))

    for k in files:
        files[k] = sorted(files[k], key=itemgetter(0))

    for k, messages in files.items():
        dest = destdir / k
        with dest.open("w") as fp:
            for x in messages:
                fp.write(x[1])
                fp.write("\n\n")
            yield dest


def extract(_):
    template = Catalog(**CATALOG_OPTIONS)

    for f in (ROOT / "en-US").rglob("*.md"):
        for lineno, message, comments, context in extract_from_file(
            extract_blocks,
            f,
        ):
            template.add(
                message,
                None,
                [(str(f.relative_to(ROOT)), lineno)],
                auto_comments=comments,
                context=context,
            )

    translations = HERE / "translations"
    dest = translations / "messages.pot"
    with dest.open("wb") as fp:
        write_po(
            fp,
            template,
            width=None,
            sort_by_file=True,
            include_lineno=True,
            ignore_obsolete=True,
        )
        print(f"{dest} writen")


def update(_):
    translations = HERE / "translations"
    with (translations / "messages.pot").open("rb") as fp:
        template = read_po(fp)

    dirs = [x for x in translations.iterdir() if x.is_dir()]
    for p in dirs:
        po_file = p / "messages.po"
        if po_file.exists():
            with po_file.open("rb") as fp:
                catalog = read_po(fp, locale=p.name, domain=po_file.name)
        else:
            catalog = Catalog(
                **CATALOG_OPTIONS,
                locale=p.name,
                domain=po_file.name,
            )

        catalog.update(template)

        with po_file.open("wb") as fp:
            write_po(
                fp,
                catalog,
                width=None,
                sort_by_file=True,
                include_lineno=True,
                include_previous=False,
            )
            print(f"{po_file} written")


def generate(_):
    translations = HERE / "translations"
    po_files = translations.glob("*/messages.po")

    with (translations / "messages.pot").open("rb") as fp:
        template = read_po(fp)
    total_strings = len(template)

    for po_file in sorted(po_files):
        code = po_file.parent.name

        if code == "en_US":
            continue

        # Write markdown files
        with po_file.open("rb") as fp:
            catalog = read_po(fp)

        nb_translated = 0
        for k, m in catalog._messages.items():
            tm = template._messages[k]
            if m.fuzzy:
                continue
            if tm.string == m.string:
                continue
            if isinstance(m.string, str) and m.string.strip() == "":
                continue
            if isinstance(m.string, tuple) and any([x.strip() == "" for x in m.string]):
                continue
            nb_translated += 1

        pct = float(nb_translated / total_strings)
        count_info = "{:>4}/{:<4} {:>4}%".format(
            nb_translated, total_strings, round(pct * 100)
        )
        if round(pct, 2) < COMPLETION_CUTOFF:
            print("[-] {:8} {}".format(code, count_info))
            continue

        destdir = HERE / "src" / str(catalog.locale_identifier).replace("_", "-")
        os.makedirs(destdir, exist_ok=True)

        nb_files = 0
        for _ in po2text(catalog, destdir):
            nb_files += 1

        print("[+] {:8} {} -- {}/".format(code, count_info, destdir.relative_to(HERE)))


def check(_):
    translations = HERE / "translations"
    po_files = translations.glob("*/messages.po")

    has_errors = False
    for filename in po_files:
        code = filename.parent.name
        if code == "en_US":
            continue

        with filename.open("rb") as fp:
            catalog = read_po(fp)

        errors = list(catalog.check())
        if len(errors) == 0:
            print(f"[OK] {code}")
        else:
            has_errors = True
            print(f"[ERRORS] {code}")
            for [m, e] in errors:
                print(f"  - #{m.lineno} - {m.id}")
                for x in e:
                    print(f"    - {str(x)}")

    sys.exit(has_errors and 1 or 0)


def main():
    parser = ArgumentParser()
    subparsers = parser.add_subparsers(required=True)

    p_extract = subparsers.add_parser("extract", help="Extract messages")
    p_extract.set_defaults(func=extract)

    p_update = subparsers.add_parser("update", help="Update strings")
    p_update.set_defaults(func=update)

    p_generate = subparsers.add_parser("generate", help="generate markdown files")
    p_generate.set_defaults(func=generate)

    p_check = subparsers.add_parser("check", help="Check translation files")
    p_check.set_defaults(func=check)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
