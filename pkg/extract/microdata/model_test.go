// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package microdata_test

import (
	"encoding/json"
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/pkg/extract/microdata"
	"github.com/stretchr/testify/require"
)

func TestModel(t *testing.T) {
	contents := `
	<ul itemscope itemtype="http://schema.org/Book" itemid="urn:isbn:978-0141196404">
		<li itemprop="title">The Black &middot; Cloud</li>
		<li itemprop="author">Fred Hoyle</li>
		<meta itemprop="description" content="Some &middot; description">
		<meta itemprop="abc" content="A &amp;middot&semi; B C">
	</ul>
	<script type="application/ld+json">
	{
		"@context": "https://schema.org",
		"@type": "Article",
		"author": [
			{
				"@type": "Person",
				"image": {
					"@type": "ImageObject",
					"height": 250,
					"url": "https://www.gravatar.com/avatar/87e07bd5bb3d003b0b135303a3e7f8b9?s=250&r=x&d=mp",
					"width": 250
				},
				"name": "Matthew Gault",
				"sameAs": ["http://matthewgault.com", "https://x.com/mjgault"],
				"url": "https://www.404media.co/author/matthew/"
			},
			{
				"@type": "Person",
				"image": {
					"@type": "ImageObject",
					"height": 1129,
					"url": "https://www.404media.co/content/images/size/w1200/2023/08/404-sam-10--1-.jpg",
					"width": 1200
				},
				"name": "Samantha Cole",
				"sameAs": ["http://samleecole.com", "https://x.com/samleecole"],
				"url": "https://www.404media.co/author/samantha-cole/"
			}
		],
		"dateModified": "2025-10-22T14:43:31.000Z",
		"datePublished": "2025-10-22T13:40:03.000Z",
		"description": "When Amazon Web Services went offline, people lost control of their cloud-connected smart beds, getting stuck in reclined positions or roasting with the heat turned all the way up.",
		"headline": "The AWS Outage Bricked People’s $2,700 Smartbeds",
		"image": {
			"@type": "ImageObject",
			"height": 800,
			"url": "https://www.404media.co/content/images/size/w1200/2025/10/PodBed-1.jpg",
			"width": 1200
		},
		"keywords": "News",
		"mainEntityOfPage": "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/",
		"publisher": {
			"@type": "Organization",
			"logo": {
				"@type": "ImageObject",
				"url": "https://www.404media.co/content/images/2023/08/Logo-Color-on-White@24x.png"
			},
			"name": "404 Media",
			"url": "https://www.404media.co/"
		},
		"url": "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/"
	}
	</script>
	`
	expectedNodes := `
	[
		{
			"type": 0,
			"path": "Book",
			"children": [
				{
					"type": 1,
					"name": "@context",
					"path": "Book.@context",
					"data": "http://schema.org"
				},
				{
					"type": 1,
					"name": "@id",
					"path": "Book.@id",
					"data": "urn:isbn:978-0141196404"
				},
				{
					"type": 1,
					"name": "@type",
					"path": "Book.@type",
					"data": "Book"
				},
				{
					"type": 1,
					"name": "abc",
					"path": "Book.abc",
					"data": "A &middot; B C"
				},
				{
					"type": 1,
					"name": "author",
					"path": "Book.author",
					"data": "Fred Hoyle"
				},
				{
					"type": 1,
					"name": "description",
					"path": "Book.description",
					"data": "Some · description"
				},
				{
					"type": 1,
					"name": "title",
					"path": "Book.title",
					"data": "The Black · Cloud"
				}
			]
		},
		{
			"type": 0,
			"path": "Article",
			"children": [
				{
					"type": 1,
					"name": "@context",
					"path": "Article.@context",
					"data": "https://schema.org"
				},
				{
					"type": 1,
					"name": "@type",
					"path": "Article.@type",
					"data": "Article"
				},
				{
					"type": 0,
					"name": "author",
					"path": "Article.author",
					"children": [
						{
							"type": 0,
							"path": "Article.author",
							"children": [
								{
									"type": 1,
									"name": "@type",
									"path": "Article.author.@type",
									"data": "Person"
								},
								{
									"type": 0,
									"name": "image",
									"path": "Article.author.image",
									"children": [
										{
											"type": 1,
											"name": "@type",
											"path": "Article.author.image.@type",
											"data": "ImageObject"
										},
										{
											"type": 1,
											"name": "height",
											"path": "Article.author.image.height",
											"data": 250
										},
										{
											"type": 1,
											"name": "url",
											"path": "Article.author.image.url",
											"data": "https://www.gravatar.com/avatar/87e07bd5bb3d003b0b135303a3e7f8b9?s=250&r=x&d=mp"
										},
										{
											"type": 1,
											"name": "width",
											"path": "Article.author.image.width",
											"data": 250
										}
									]
								},
								{
									"type": 1,
									"name": "name",
									"path": "Article.author.name",
									"data": "Matthew Gault"
								},
								{
									"type": 0,
									"name": "sameAs",
									"path": "Article.author.sameAs",
									"children": [
										{
											"type": 1,
											"path": "Article.author.sameAs",
											"data": "http://matthewgault.com"
										},
										{
											"type": 1,
											"path": "Article.author.sameAs",
											"data": "https://x.com/mjgault"
										}
									]
								},
								{
									"type": 1,
									"name": "url",
									"path": "Article.author.url",
									"data": "https://www.404media.co/author/matthew/"
								}
							]
						},
						{
							"type": 0,
							"path": "Article.author",
							"children": [
								{
									"type": 1,
									"name": "@type",
									"path": "Article.author.@type",
									"data": "Person"
								},
								{
									"type": 0,
									"name": "image",
									"path": "Article.author.image",
									"children": [
										{
											"type": 1,
											"name": "@type",
											"path": "Article.author.image.@type",
											"data": "ImageObject"
										},
										{
											"type": 1,
											"name": "height",
											"path": "Article.author.image.height",
											"data": 1129
										},
										{
											"type": 1,
											"name": "url",
											"path": "Article.author.image.url",
											"data": "https://www.404media.co/content/images/size/w1200/2023/08/404-sam-10--1-.jpg"
										},
										{
											"type": 1,
											"name": "width",
											"path": "Article.author.image.width",
											"data": 1200
										}
									]
								},
								{
									"type": 1,
									"name": "name",
									"path": "Article.author.name",
									"data": "Samantha Cole"
								},
								{
									"type": 0,
									"name": "sameAs",
									"path": "Article.author.sameAs",
									"children": [
										{
											"type": 1,
											"path": "Article.author.sameAs",
											"data": "http://samleecole.com"
										},
										{
											"type": 1,
											"path": "Article.author.sameAs",
											"data": "https://x.com/samleecole"
										}
									]
								},
								{
									"type": 1,
									"name": "url",
									"path": "Article.author.url",
									"data": "https://www.404media.co/author/samantha-cole/"
								}
							]
						}
					]
				},
				{
					"type": 1,
					"name": "dateModified",
					"path": "Article.dateModified",
					"data": "2025-10-22T14:43:31.000Z"
				},
				{
					"type": 1,
					"name": "datePublished",
					"path": "Article.datePublished",
					"data": "2025-10-22T13:40:03.000Z"
				},
				{
					"type": 1,
					"name": "description",
					"path": "Article.description",
					"data": "When Amazon Web Services went offline, people lost control of their cloud-connected smart beds, getting stuck in reclined positions or roasting with the heat turned all the way up."
				},
				{
					"type": 1,
					"name": "headline",
					"path": "Article.headline",
					"data": "The AWS Outage Bricked People’s $2,700 Smartbeds"
				},
				{
					"type": 0,
					"name": "image",
					"path": "Article.image",
					"children": [
						{
							"type": 1,
							"name": "@type",
							"path": "Article.image.@type",
							"data": "ImageObject"
						},
						{
							"type": 1,
							"name": "height",
							"path": "Article.image.height",
							"data": 800
						},
						{
							"type": 1,
							"name": "url",
							"path": "Article.image.url",
							"data": "https://www.404media.co/content/images/size/w1200/2025/10/PodBed-1.jpg"
						},
						{
							"type": 1,
							"name": "width",
							"path": "Article.image.width",
							"data": 1200
						}
					]
				},
				{
					"type": 1,
					"name": "keywords",
					"path": "Article.keywords",
					"data": "News"
				},
				{
					"type": 1,
					"name": "mainEntityOfPage",
					"path": "Article.mainEntityOfPage",
					"data": "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/"
				},
				{
					"type": 0,
					"name": "publisher",
					"path": "Article.publisher",
					"children": [
						{
							"type": 1,
							"name": "@type",
							"path": "Article.publisher.@type",
							"data": "Organization"
						},
						{
							"type": 0,
							"name": "logo",
							"path": "Article.publisher.logo",
							"children": [
								{
									"type": 1,
									"name": "@type",
									"path": "Article.publisher.logo.@type",
									"data": "ImageObject"
								},
								{
									"type": 1,
									"name": "url",
									"path": "Article.publisher.logo.url",
									"data": "https://www.404media.co/content/images/2023/08/Logo-Color-on-White@24x.png"
								}
							]
						},
						{
							"type": 1,
							"name": "name",
							"path": "Article.publisher.name",
							"data": "404 Media"
						},
						{
							"type": 1,
							"name": "url",
							"path": "Article.publisher.url",
							"data": "https://www.404media.co/"
						}
					]
				},
				{
					"type": 1,
					"name": "url",
					"path": "Article.url",
					"data": "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/"
				}
			]
		}
	]
	`

	t.Run("render json", runParseHTML(contents, func(t *testing.T, md *microdata.Microdata) {
		out := new(strings.Builder)
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		require.NoError(t, enc.Encode(md.Nodes))

		require.JSONEq(t, expectedNodes, out.String())
	}))

	t.Run("iter props", runParseHTML(contents, func(t *testing.T, md *microdata.Microdata) {
		expected := [][2]any{
			{"Book.@context", "http://schema.org"},
			{"Book.@id", "urn:isbn:978-0141196404"},
			{"Book.@type", "Book"},
			{"Book.abc", "A &middot; B C"},
			{"Book.author", "Fred Hoyle"},
			{"Book.description", "Some · description"},
			{"Book.title", "The Black · Cloud"},
			{"Article.@context", "https://schema.org"},
			{"Article.@type", "Article"},
			{"Article.author.@type", "Person"},
			{"Article.author.image.@type", "ImageObject"},
			{"Article.author.image.height", 250},
			{"Article.author.image.url", "https://www.gravatar.com/avatar/87e07bd5bb3d003b0b135303a3e7f8b9?s=250&r=x&d=mp"},
			{"Article.author.image.width", 250},
			{"Article.author.name", "Matthew Gault"},
			{"Article.author.sameAs", "http://matthewgault.com"},
			{"Article.author.sameAs", "https://x.com/mjgault"},
			{"Article.author.url", "https://www.404media.co/author/matthew/"},
			{"Article.author.@type", "Person"},
			{"Article.author.image.@type", "ImageObject"},
			{"Article.author.image.height", 1129},
			{"Article.author.image.url", "https://www.404media.co/content/images/size/w1200/2023/08/404-sam-10--1-.jpg"},
			{"Article.author.image.width", 1200},
			{"Article.author.name", "Samantha Cole"},
			{"Article.author.sameAs", "http://samleecole.com"},
			{"Article.author.sameAs", "https://x.com/samleecole"},
			{"Article.author.url", "https://www.404media.co/author/samantha-cole/"},
			{"Article.dateModified", "2025-10-22T14:43:31.000Z"},
			{"Article.datePublished", "2025-10-22T13:40:03.000Z"},
			{"Article.description", "When Amazon Web Services went offline, people lost control of their cloud-connected smart beds, getting stuck in reclined positions or roasting with the heat turned all the way up."},
			{"Article.headline", "The AWS Outage Bricked People’s $2,700 Smartbeds"},
			{"Article.image.@type", "ImageObject"},
			{"Article.image.height", 800},
			{"Article.image.url", "https://www.404media.co/content/images/size/w1200/2025/10/PodBed-1.jpg"},
			{"Article.image.width", 1200},
			{"Article.keywords", "News"},
			{"Article.mainEntityOfPage", "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/"},
			{"Article.publisher.@type", "Organization"},
			{"Article.publisher.logo.@type", "ImageObject"},
			{"Article.publisher.logo.url", "https://www.404media.co/content/images/2023/08/Logo-Color-on-White@24x.png"},
			{"Article.publisher.name", "404 Media"},
			{"Article.publisher.url", "https://www.404media.co/"},
			{"Article.url", "https://www.404media.co/the-aws-outage-bricked-peoples-2-700-smartbeds/"},
		}

		props := [][2]any{}
		for node := range md.All(func(n *microdata.Node) bool {
			return n.Type == microdata.Property
		}) {
			props = append(props, [2]any{node.Path, node.Data})
		}
		require.Equal(t, expected, props)

		// checking that an iteration stop doen't panic
		require.NotPanics(t, func() {
			for range md.All(nil) {
				return
			}
		})
	}))
}
