// The light a picture throws on the wall behind it.
//
// The home hero and the library's first card are large pictures, and the page
// under them is a near-black that made them sit on it like posters on a wall.
// This reads an artwork's own pixels and answers the one or two colours that
// stand out in it; the stylesheet spreads them around the picture and drops them
// back to the page's ink with distance (design system 6b).
//
// The read goes through a canvas, which needs the picture to be CORS-clean: the
// server sends Access-Control-Allow-Origin for /api/images - the same headers
// the shell's artwork needed - and every artwork <img> here asks with
// crossOrigin. When the pixels cannot be read (a refused origin, a picture the
// CDN answers without the header), the read throws and the answer is "no
// colour": a page with no glow rather than a page that breaks.

type Channels = string;

const CACHE = new Map<string, Promise<Channels[] | null>>();

/**
 * Answers up to two colours for a picture as "r g b" channel strings - the form
 * the stylesheet puts inside `rgb(... / alpha)`. One is the common answer; a
 * second appears when the picture really has a second colour in it.
 */
export function ambientColours(url: string | null | undefined): Promise<Channels[] | null> {
	if (!url) return Promise.resolve(null);
	const remembered = CACHE.get(url);
	if (remembered) return remembered;
	const pending = read(url).catch(() => null);
	CACHE.set(url, pending);
	return pending;
}

async function read(url: string): Promise<Channels[] | null> {
	const image = await load(url);
	// Thirty-two by eighteen pixels is enough to answer "which colour is this
	// film": film stills are graded, not speckled, and the sample stays at a few
	// hundred pixels to read.
	const canvas = document.createElement('canvas');
	canvas.width = 32;
	canvas.height = 18;
	const context = canvas.getContext('2d', { willReadFrequently: true });
	if (!context) return null;
	context.drawImage(image, 0, 0, canvas.width, canvas.height);
	return pick(context.getImageData(0, 0, canvas.width, canvas.height).data);
}

function load(url: string): Promise<HTMLImageElement> {
	// `new Promise` rather than `Promise.withResolvers`: that one is ES2024 and
	// this application compiles against ES2022 (player/ui/tsconfig.json), so the
	// newer form would be a claim about every WebView in the world that this
	// project has not measured.
	return new Promise((resolve, reject) => {
		const image = new Image();
		image.crossOrigin = 'anonymous';
		image.onload = () => resolve(image);
		image.onerror = () => reject(new Error('the artwork could not be drawn'));
		image.src = url;
	});
}

type Bucket = { red: number; green: number; blue: number; weight: number };

// pick buckets the pixels and answers the loudest colours in them, weighted so a
// saturated patch outranks a larger grey one. That is the point: the most common
// colour of a film still is the dark the frame is mostly made of, and a glow in
// that colour is a glow about nothing.
function pick(pixels: Uint8ClampedArray): Channels[] | null {
	const buckets = new Map<number, Bucket>();
	for (let index = 0; index < pixels.length; index += 4) {
		const red = pixels[index];
		const green = pixels[index + 1];
		const blue = pixels[index + 2];
		if (pixels[index + 3] < 200) continue;
		const max = Math.max(red, green, blue);
		const min = Math.min(red, green, blue);
		// The letterboxing and the blown highlights are the frame, not the light.
		if (max < 26 || min > 236) continue;
		const saturation = max === 0 ? 0 : (max - min) / max;
		const key = ((red >> 4) << 8) | ((green >> 4) << 4) | (blue >> 4);
		const bucket = buckets.get(key) ?? { red: 0, green: 0, blue: 0, weight: 0 };
		const weight = 0.35 + saturation;
		bucket.red += red * weight;
		bucket.green += green * weight;
		bucket.blue += blue * weight;
		bucket.weight += weight;
		buckets.set(key, bucket);
	}
	if (buckets.size === 0) return null;

	const ranked = [...buckets.values()]
		.map((bucket) => ({
			colour: [bucket.red / bucket.weight, bucket.green / bucket.weight, bucket.blue / bucket.weight],
			weight: bucket.weight,
		}))
		.sort((left, right) => right.weight - left.weight)
		.map((entry) => usable(entry.colour));

	// The second colour has to be a second colour: two neighbours of one hue
	// would draw the same circle twice.
	const first = ranked[0];
	const second = ranked.find(
		(colour) => Math.hypot(colour[0] - first[0], colour[1] - first[1], colour[2] - first[2]) > 96
	);
	return [first, second]
		.filter((colour): colour is number[] => Boolean(colour))
		.map((colour) => colour.map((channel) => Math.round(Math.min(255, Math.max(0, channel)))).join(' '));
}

// usable lifts a colour into the band a glow reads in: a nearly black dominant
// cannot light anything, and a saturated one does not need the help.
function usable(colour: number[]): number[] {
	const [red, green, blue] = colour.map((channel) => channel / 255);
	const max = Math.max(red, green, blue);
	const min = Math.min(red, green, blue);
	const lightness = (max + min) / 2;
	const spread = max - min;
	const saturation = spread === 0 ? 0 : spread / (lightness > 0.5 ? 2 - max - min : max + min);
	return hsl(hueOf(red, green, blue, max, spread), Math.min(0.82, Math.max(0.28, saturation)), Math.min(0.66, Math.max(0.34, lightness)));
}

function hueOf(red: number, green: number, blue: number, max: number, spread: number): number {
	if (spread === 0) return 0;
	const raw =
		max === red ? (green - blue) / spread : max === green ? (blue - red) / spread + 2 : (red - green) / spread + 4;
	return ((((raw % 6) + 6) % 6) / 6);
}

function hsl(hue: number, saturation: number, lightness: number): number[] {
	const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
	const second = chroma * (1 - Math.abs(((hue * 6) % 2) - 1));
	const base = lightness - chroma / 2;
	const parts = [
		[chroma, second, 0],
		[second, chroma, 0],
		[0, chroma, second],
		[0, second, chroma],
		[second, 0, chroma],
		[chroma, 0, second],
	][Math.floor(hue * 6) % 6];
	return parts.map((part) => (part + base) * 255);
}
