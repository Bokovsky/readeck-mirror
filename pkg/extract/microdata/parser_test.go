// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package microdata_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/pkg/extract/microdata"
)

func runParseHTML(src string, f func(t *testing.T, md *microdata.Microdata)) func(t *testing.T) {
	return func(t *testing.T) {
		root, err := html.Parse(strings.NewReader(src))
		require.NoError(t, err)

		md, err := microdata.ParseNode(root, "https://example.org/")
		require.NoError(t, err)

		f(t, md)
	}
}

func runParseAndEncodeRaw(src string, expected string) func(t *testing.T) {
	return runParseHTML(src, func(t *testing.T, md *microdata.Microdata) {
		out := new(strings.Builder)
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		require.NoError(t, enc.Encode(md.Raw()))

		require.JSONEq(t, expected, out.String())
	})
}

func TestParserSchemaOrg(t *testing.T) {
	// nolint:misspell
	tests := []struct {
		html     string
		expected string
	}{
		{
			`
			<div itemscope itemtype="https://schema.org/Movie">
			<h1 itemprop="name">Pirates of the Carribean: On Stranger Tides (2011)</h1>
			<span itemprop="description">Jack Sparrow and Barbossa embark on a quest to
			find the elusive fountain of youth, only to discover that Blackbeard and
			his daughter are after it too.</span>
			Director:
			<div itemprop="director" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Rob Marshall</span>
			</div>
			Writers:
			<div itemprop="author" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Ted Elliott</span>
			</div>
			<div itemprop="author" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Terry Rossio</span>
			</div>
			, and 7 more credits
			Stars:
			<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Johnny Depp</span>,
			</div>
			<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Penelope Cruz</span>,
			</div>
			<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
			<span itemprop="name">Ian McShane</span>
			</div>
			<div itemprop="aggregateRating" itemscope itemtype="https://schema.org/AggregateRating">
			<span itemprop="ratingValue">8</span>/<span itemprop="bestRating">10</span> stars from
			<span itemprop="ratingCount">200</span> users.
			Reviews: <span itemprop="reviewCount">50</span>.
			</div>
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "Movie",
				"actor": [
					{
					"@type": "Person",
					"name": "Johnny Depp"
					},
					{
					"@type": "Person",
					"name": "Penelope Cruz"
					},
					{
					"@type": "Person",
					"name": "Ian McShane"
					}
				],
				"aggregateRating": {
					"@type": "AggregateRating",
					"bestRating": "10",
					"ratingCount": "200",
					"ratingValue": "8",
					"reviewCount": "50"
				},
				"author": [
					{
					"@type": "Person",
					"name": "Ted Elliott"
					},
					{
					"@type": "Person",
					"name": "Terry Rossio"
					}
				],
				"description": "Jack Sparrow and Barbossa embark on a quest to find the elusive fountain of youth, only to discover that Blackbeard and his daughter are after it too.",
				"director": {
					"@type": "Person",
					"name": "Rob Marshall"
				},
				"name": "Pirates of the Carribean: On Stranger Tides (2011)"
			}]`,
		},
		{
			`
			<div itemscope itemtype="https://schema.org/Movie">
			<meta itemprop="name" content="Footloose">
			<div itemprop="potentialAction" itemscope itemtype="https://schema.org/WatchAction">
				<a itemprop="target" href="http://example.com/player?id=123">Watch <cite>Footloose</cite></a>
			</div>
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "Movie",
				"name": "Footloose",
				"potentialAction" : {
					"@type": "WatchAction",
					"target" : "http://example.com/player?id=123"
				}
			}]`,
		},
		{
			`
			<div itemscope itemtype="https://schema.org/ScreeningEvent">
			<h1 itemprop="name">Jaws 3-D</h1>
			<div itemprop="description">Jaws 3-D shown in 3D.</div>
			<p>Location: <span itemprop="location" itemscope itemtype="https://schema.org/MovieTheater">
				<span itemprop="name">ACME Cinemas 10</span>
				<span itemprop="screenCount">10</span>
				</span>
			</p>
			<div itemprop="workPresented" itemscope itemtype="https://schema.org/Movie">
				<span itemprop="name">Jaws 3-D</span>
				<link itemprop="sameAs" href="https://www.imdb.com/title/tt0085750/"/>
			</div>
			<p>Language: <span itemprop="inLanguage" content="en">English</span></p>
			<p>Film format: <span itemprop="videoFormat">3D</span></p>
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "ScreeningEvent",
				"name": "Jaws 3-D",
				"description": "Jaws 3-D shown in 3D.",
				"location": {
					"@type": "MovieTheater",
					"name": "ACME Cinemas 10",
					"screenCount": "10"
				},
				"workPresented": {
					"@type": "Movie",
					"name": "Jaws 3-D",
					"sameAs": "https://www.imdb.com/title/tt0085750/"
				},
				"inLanguage": "en",
				"videoFormat": "3D"
			}]`,
		},
		{
			`
			<p>
			This example shows the addition of Accessibility metadata. Although these properties are not
			a formal enumeration, there is evolving consensus amongst accessibility experts for
			appropriate values for these properties. This example shows simple text values,
			as suggested by www.a11ymetadata.org.
			</p>

			<div itemscope="" itemtype="https://schema.org/Book">
			<meta itemprop="bookFormat" content="EBook/DAISY3"/>
			<meta itemprop="accessibilityFeature" content="largePrint/CSSEnabled"/>
			<meta itemprop="accessibilityFeature" content="highContrast/CSSEnabled"/>
			<meta itemprop="accessibilityFeature" content="resizeText/CSSEnabled"/>
			<meta itemprop="accessibilityFeature" content="displayTransformability"/>
			<meta itemprop="accessibilityFeature" content="longDescription"/>
			<meta itemprop="accessibilityFeature" content="alternativeText"/>
			<meta itemprop="accessibilityControl" content="fullKeyboardControl"/>
			<meta itemprop="accessibilityControl" content="fullMouseControl"/>
			<meta itemprop="accessibilityHazard" content="noFlashingHazard"/>
			<meta itemprop="accessibilityHazard" content="noMotionSimulationHazard"/>
			<meta itemprop="accessibilityHazard" content="noSoundHazard"/>
			<meta itemprop="accessibilityAPI" content="ARIA"/>

			<dl>
				<dt>Name:</dt>
				<dd itemprop="name">Holt Physical Science</dd>
				<dt>Brief Synopsis:</dt>
				<dd itemprop="description">NIMAC-sourced textbook</dd>
				<dt>Long Synopsis:</dt>
				<dd>N/A</dd>
				<dt>Book Quality:</dt>
				<dd>Publisher Quality</dd>
				<dt>Book Size:</dt>
				<dd><span itemprop="numberOfPages">598</span> Pages</dd>
				<dt>ISBN-13:</dt>
				<dd itemprop="isbn">9780030426599</dd>
				<dt>Publisher:</dt>
				<dd itemprop="publisher" itemtype="https://schema.org/Organization" itemscope=""><span itemprop="name">Holt, Rinehart and Winston</span></dd>
				<dt>Date of Addition:</dt>
				<dd>06/08/10</dd>
				<dt>Copyright Date:</dt>
				<dd itemprop="copyrightYear">2007</dd>
				<dt>Copyrighted By:</dt>
				<dd itemprop="copyrightHolder" itemtype="https://schema.org/Organization" itemscope=""><span itemprop="name">Holt, Rinehart and Winston</span></dd>
				<dt>Adult content:</dt>
				<dd><meta itemprop="isFamilyFriendly" content="true"/>No</dd>
				<dt>Language:</dt>
				<dd><meta itemprop="inLanguage" content="en-US"/>English US</dd>
				<dt>Essential Images:</dt>
				<dd>861</dd>
				<dt>Described Images:</dt>
				<dd>910</dd>
				<dt>Categories:</dt>
				<dd><span itemprop="genre">Educational Materials</span></dd>
				<dt>Grade Levels:</dt>
				<dd>Sixth grade, Seventh grade, Eighth grade</dd>
				<dt>Submitted By:</dt>
				<dd>Bookshare Staff</dd>
				<dt>NIMAC:</dt>
				<dd>This book is currently only available to public K-12 schools and organizations in the
				United States for use with students with an IEP, because it was created from files
				supplied by the NIMAC under these restrictions. Learn more in the NIMAC Support Center.</dd>
			</dl>

			<div class="bookReviews" itemprop="aggregateRating" itemscope itemtype="https://schema.org/AggregateRating">
				<h2>Reviews of Holt Physical Science (<span itemprop="reviewCount">0</span> reviews)</h2>

				<div class="bookReviewScore">
					<span><span itemprop="ratingValue">0</span> - No Rating Yet</span>
				</div>
			</div>
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "Book",
				"accessibilityAPI": "ARIA",
				"accessibilityControl": [
					"fullKeyboardControl",
					"fullMouseControl"
				],
				"accessibilityFeature": [
					"largePrint/CSSEnabled",
					"highContrast/CSSEnabled",
					"resizeText/CSSEnabled",
					"displayTransformability",
					"longDescription",
					"alternativeText"
				],
				"accessibilityHazard": [
					"noFlashingHazard",
					"noMotionSimulationHazard",
					"noSoundHazard"
				],
				"aggregateRating": {
					"@type": "AggregateRating",
					"reviewCount": "0",
					"ratingValue": "0"
				},
				"bookFormat": "EBook/DAISY3",
				"copyrightHolder": {
					"@type": "Organization",
					"name": "Holt, Rinehart and Winston"
				},
				"copyrightYear": "2007",
				"description": "NIMAC-sourced textbook",
				"genre": "Educational Materials",
				"inLanguage": "en-US",
				"isFamilyFriendly": "true",
				"isbn": "9780030426599",
				"name": "Holt Physical Science",
				"numberOfPages": "598",
				"publisher": {
					"@type": "Organization",
					"name": "Holt, Rinehart and Winston"
				}
			}]`,
		},
		{
			`
			<div itemscope itemtype="https://schema.org/Periodical">
			<h1 itemprop="name">The Lancet</h1>
			<p>Volume 376, July 2010-December 2010</p>
			<p>Published by <span itemprop="publisher">Elsevier</span>
			<ul>
				<li>ISSN <span itemprop="issn">0140-6736</span></li>
			</ul>
			<h3>Issues:</h3>
			<div itemprop="hasPart" itemscope itemtype="https://schema.org/PublicationVolume" itemid="#vol376">
				<meta itemprop="volumeNumber" content="376">
				<ul>
				<li itemprop="hasPart" itemscope itemtype="https://schema.org/PublicationIssue" itemid="#iss9734">No.
					<span itemprop="issueNumber">9734</span>
					<time datetime="2010-07-03" itemprop="datePublished">Jul 3, 2010</time>
					p <span itemprop="pageStart">1</span>-<span itemprop="pageEnd">68</span>
				</li>
				<li itemprop="hasPart" itemscope itemtype="https://schema.org/PublicationIssue" itemid="#iss9735">No.
					<span itemprop="issueNumber">9735</span>
					<time datetime="2010-07-03" itemprop="datePublished">Jul 10, 2010</time>
					p <span itemprop="pageStart">69</span>-<span itemprop="pageEnd">140</span>
				</li>
				</ul>
			</div>
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "Periodical",
				"issn": "0140-6736",
				"hasPart": {
					"@id": "vol376",
					"@type": "PublicationVolume",
					"volumeNumber": "376",
					"hasPart": [
						{
							"@id": "iss9734",
							"@type": "PublicationIssue",
							"datePublished": "2010-07-03",
							"pageEnd": "68",
							"pageStart": "1",
							"issueNumber": "9734"
						},
						{
							"@id": "iss9735",
							"@type": "PublicationIssue",
							"datePublished": "2010-07-03",
							"pageEnd": "140",
							"pageStart": "69",
							"issueNumber": "9735"
						}
					]
				},
				"name": "The Lancet",
				"publisher": "Elsevier"
			}]`,
		},
		{
			`
			<div itemscope itemtype="https://schema.org/Recipe">
			<span itemprop="name">Mom's World Famous Banana Bread</span>
			By <span itemprop="author">John Smith</span>,
			<meta itemprop="datePublished" content="2009-05-08">May 8, 2009
			<img itemprop="image" src="bananabread.jpg"
				alt="Banana bread on a plate" />

			<span itemprop="description">This classic banana bread recipe comes
			from my mom -- the walnuts add a nice texture and flavor to the banana
			bread.</span>

			Prep Time: <meta itemprop="prepTime" content="PT15M">15 minutes
			Cook time: <meta itemprop="cookTime" content="PT1H">1 hour
			Yield: <span itemprop="recipeYield">1 loaf</span>
			Tags: <link itemprop="suitableForDiet" href="https://schema.org/LowFatDiet" />Low fat

			<div itemprop="nutrition"
				itemscope itemtype="https://schema.org/NutritionInformation">
				Nutrition facts:
				<span itemprop="calories">240 calories</span>,
				<span itemprop="fatContent">9 grams fat</span>
			</div>

			Ingredients:
			- <span itemprop="recipeIngredient">3 or 4 ripe bananas, smashed</span>
			- <span itemprop="recipeIngredient" itemscope itemtype="https://schema.org/PropertyValue"><span itemprop="value">1</span> <span itemprop="name">egg</span></span>
			- <span itemprop="recipeIngredient" itemscope itemtype="https://schema.org/PropertyValue"><span itemprop="value">3/4</span> <span itemprop="unitCode">cup</span> of <span itemprop="name">sugar</span></span>
			...

			Instructions:
			<span itemprop="recipeInstructions">
			Preheat the oven to 350 degrees. Mix in the ingredients in a bowl. Add
			the flour last. Pour the mixture into a loaf pan and bake for one hour.
			</span>

			140 comments:
			<div itemprop="interactionStatistic" itemscope itemtype="https://schema.org/InteractionCounter">
				<meta itemprop="interactionType" content="https://schema.org/CommentAction" />
				<meta itemprop="userInteractionCount" content="140" />
			</div>
			From Janel, May 5 -- thank you, great recipe!
			...
			</div>
			`,
			`[{
				"@context": "https://schema.org",
				"@type": "Recipe",
				"author": "John Smith",
				"cookTime": "PT1H",
				"datePublished": "2009-05-08",
				"description": "This classic banana bread recipe comes from my mom -- the walnuts add a nice texture and flavor to the banana bread.",
				"image": "https://example.org/bananabread.jpg",
				"recipeIngredient": [
					"3 or 4 ripe bananas, smashed",
					{ "@type": "PropertyValue", "value": "1", "name": "egg" },
					{ "@type": "PropertyValue", "value": "3/4", "name": "sugar", "unitCode": "cup"}
				],
				"interactionStatistic": {
					"@type": "InteractionCounter",
					"interactionType": "https://schema.org/CommentAction",
					"userInteractionCount": "140"
				},
				"name": "Mom's World Famous Banana Bread",
				"nutrition": {
					"@type": "NutritionInformation",
					"calories": "240 calories",
					"fatContent": "9 grams fat"
				},
				"prepTime": "PT15M",
				"recipeInstructions": "Preheat the oven to 350 degrees. Mix in the ingredients in a bowl. Add the flour last. Pour the mixture into a loaf pan and bake for one hour.",
				"recipeYield": "1 loaf",
				"suitableForDiet": "https://schema.org/LowFatDiet"
			}]`,
		},
		{
			`
			<html>
			<body>
				<div itemscope itemtype="http://schema.org/Movie">
					<h1 itemprop="name">Avatar</h1>
					<div itemprop="director" itemscope itemtype="http://schema.org/Person">
						Director: <span itemprop="name">James Cameron</span>
						(born <time itemprop="birthDate" datetime="1954-08-16">August 16, 1954</time>)
					</div>
					<div>
						<span itemprop="genre">Action</span>
					</div>
					<span itemprop="genre">Fiction</span>
					<span itemprop="genre">Science fiction</span>
					<a href="../movies/avatar-theatrical-trailer.html" itemprop="trailer">Trailer</a>
				</div>

				<div itemscope itemtype="https://schema.org/Movie">
					<h1 itemprop="name">Pirates of the Carribean: On Stranger Tides (2011)</h1>
					<span itemprop="description">Jack Sparrow and Barbossa embark on a quest to
					find the elusive fountain of youth, only to discover that Blackbeard and
					his daughter are after it too.</span>
					Director:
					<div itemprop="director" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Rob Marshall</span>
					</div>
					Writers:
					<div itemprop="author" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Ted Elliott</span>
					</div>
					<div itemprop="author" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Terry Rossio</span>
					</div>
					, and 7 more credits
					Stars:
					<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Johnny Depp</span>,
					</div>
					<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Penelope Cruz</span>,
					</div>
					<div itemprop="actor" itemscope itemtype="https://schema.org/Person">
						<span itemprop="name">Ian McShane</span>
					</div>
					<div itemprop="aggregateRating" itemscope itemtype="https://schema.org/AggregateRating">
						<span itemprop="ratingValue">8</span>/<span itemprop="bestRating">10</span> stars from
						<span itemprop="ratingCount">200</span> users.
						Reviews: <span itemprop="reviewCount">50</span>.
					</div>
				</div>
			</body>
			</html>
			`,
			`[
				{
					"@context": "http://schema.org",
					"@type": "Movie",
					"director": {
						"@type": "Person",
						"birthDate": "1954-08-16",
						"name": "James Cameron"
					},
					"genre": ["Action", "Fiction", "Science fiction"],
					"name": "Avatar",
					"trailer": "https://example.org/movies/avatar-theatrical-trailer.html"
				},
				{
					"@context": "https://schema.org",
					"@type": "Movie",
					"actor": [
						{
							"@type": "Person",
							"name": "Johnny Depp"
						},
						{
							"@type": "Person",
							"name": "Penelope Cruz"
						},
						{
							"@type": "Person",
							"name": "Ian McShane"
						}
					],
					"aggregateRating": {
						"@type": "AggregateRating",
						"bestRating": "10",
						"ratingCount": "200",
						"ratingValue": "8",
						"reviewCount": "50"
					},
					"author": [
						{
							"@type": "Person",
							"name": "Ted Elliott"
						},
						{
							"@type": "Person",
							"name": "Terry Rossio"
						}
					],
					"description": "Jack Sparrow and Barbossa embark on a quest to find the elusive fountain of youth, only to discover that Blackbeard and his daughter are after it too.",
					"director": {
						"@type": "Person",
						"name": "Rob Marshall"
					},
					"name": "Pirates of the Carribean: On Stranger Tides (2011)"
				}
			]`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), runParseAndEncodeRaw(test.html, test.expected))
	}
}

func TestMicrodataFeatures(t *testing.T) {
	// nolint:misspell
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			"item scope",
			`
			<div itemscope itemtype="http://schema.org/Person">
				<p>My name is <span itemprop="name">Penelope</span>.</p>
			</div>
			`,
			`[{
				"@context": "http://schema.org",
				"@type": "Person",
				"name": "Penelope"
			}]`,
		},
		{
			"item ref",
			`
			<div itemscope itemtype="http://schema.org/Movie" itemref="properties">
				<p><span itemprop="name">Rear Window</span> is a movie from 1954.</p>
			</div>
			<ul id="properties">
				<li itemprop="genre">Thriller</li>
				<li itemprop="description">A homebound photographer spies on his neighbours.</li>
			</ul>
			`,
			`[{
				"@context": "http://schema.org",
				"@type": "Movie",
				"description": "A homebound photographer spies on his neighbours.",
				"genre": "Thriller",
				"name": "Rear Window"
			}]`,
		},
		{
			"item id",
			`
			<ul itemscope itemtype="http://schema.org/Book" itemid="urn:isbn:978-0141196404">
				<li itemprop="title">The Black Cloud</li>
				<li itemprop="author">Fred Hoyle</li>
			</ul>
			`,
			`[{
				"@context": "http://schema.org",
				"@id": "urn:isbn:978-0141196404",
				"@type": "Book",
				"author": "Fred Hoyle",
				"title": "The Black Cloud"
			}]`,
		},
		{
			"meta content",
			`
			<html itemscope itemtype="http://schema.org/Person">
				<meta itemprop="length" content="1.70" />
			</html>
			`,
			`[{
				"@context": "http://schema.org",
				"@type": "Person",
				"length": "1.70"
			}]`,
		},
		{
			"parse value",
			`
			<div itemscope itemtype="http://schema.org/Container">
				<data itemprop="capacity" value="80">80 liters</data>
				<meter itemprop="volume" min="0" max="100" value="25">25%</meter>
			</div>
			`,
			`[{
				"@context": "http://schema.org",
				"@type": "Container",
				"capacity": "80",
				"volume": "25"
			}]`,
		},
		{
			"unescape html",
			`
			<ul itemscope itemtype="http://schema.org/Book" itemid="urn:isbn:978-0141196404">
				<li itemprop="title">The Black &middot; Cloud</li>
				<li itemprop="author">Fred Hoyle</li>
				<meta itemprop="description" content="Some &middot; description">
				<meta itemprop="abc" content="A &amp;middot&semi; B C">
			</ul>
			<script type="application/ld+json">
			{
				"@context": "http://schema.org",
				"@id": "urn:isbn:978-0141196404",
				"@type": "Book",
				"author": "Fred Hoyle",
				"title": "The Black &middot; Cloud",
				"description": "Some &middot; description",
				"abc": "A &amp;middot&semi; B C"
			}
			</script>
			`,
			`[
				{
					"@context": "http://schema.org",
					"@id": "urn:isbn:978-0141196404",
					"@type": "Book",
					"author": "Fred Hoyle",
					"title": "The Black · Cloud",
					"description": "Some · description",
					"abc": "A &middot; B C"
				},
				{
					"@context": "http://schema.org",
					"@id": "urn:isbn:978-0141196404",
					"@type": "Book",
					"author": "Fred Hoyle",
					"title": "The Black · Cloud",
					"description": "Some · description",
					"abc": "A &middot; B C"
				}
			]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, runParseAndEncodeRaw(test.html, test.expected))
	}
}

func TestInvalidJSON(t *testing.T) {
	// nolint:misspell
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			"unescape html",
			`
			<script type="application/ld+json">
			{
				"@context": "http://schema.org",
				"@id": "urn:isbn:978-0141196404",
				"@type": "Book",
				"author": "Fred Hoyle",
				"title": "The Black &middot; Cloud",
				"description": "Some &middot; description",
				// Comment, trailing coma and double escaping
				"abc": "A &amp;middot&semi; B C",
			}
			</script>
			`,
			`[{
				"@context": "http://schema.org",
				"@id": "urn:isbn:978-0141196404",
				"@type": "Book",
				"author": "Fred Hoyle",
				"title": "The Black · Cloud",
				"description": "Some · description",
				"abc": "A &middot; B C"
			}]`,
		},
	}

	for _, test := range tests {
		root, err := html.Parse(strings.NewReader(test.html))
		require.NoError(t, err)

		md, err := microdata.ParseNode(root, "https://example.org/")
		require.Error(t, err)
		require.Nil(t, md)
	}
}
