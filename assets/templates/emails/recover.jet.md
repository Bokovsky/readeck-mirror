{*
SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>

SPDX-License-Identifier: AGPL-3.0-only
*}
{{- if isset(.RecoverLink) -}}
{{- gettext(`
Hi,

You (or someone else) entered this email address when trying to
change the password of a Readeck account (%[1]s).

If you are expecting this email, please follow this link to set
a new password for your readeck account.

%[2]s
`, .SiteURL, .RecoverLink)|unsafe() -}}
{{- else -}}
{{- gettext(`
Hi,

You (or someone else) entered this email address when trying to
change the password of a Readeck account (%[1]s).

However, this email address is not associated with any account and
therefore, the attempted password change has failed.

If you are a Readeck user on %[1]s and you are
expecting this email, please try again using the email address
you used when creating your account.

If you are not a Readeck user, please ignore this message.
`, .SiteURL)|unsafe() -}}
{{- end -}}
