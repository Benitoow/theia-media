import { test } from 'node:test';
import assert from 'node:assert/strict';
import { initializationMIME } from '../../src/lib/media-transport.js';
const record = (type, payload) => {
 const bytes = new Uint8Array(8 + payload.length);
 new DataView(bytes.buffer).setUint32(0, bytes.length);
 bytes.set(new TextEncoder().encode(type), 4); bytes.set(payload, 8);
 return bytes;
};
const concat = (...items) => Uint8Array.from(items.flatMap(x => [...x]));
test('AVC profile and AAC come from the initialization segment', () => {
 assert.equal(initializationMIME(concat(record('avcC', [1,100,0,41,0,0]),record('mp4a',Array(12).fill(0)))), 'video/mp4; codecs="avc1.640029, mp4a.40.2"');
});
test('HEVC profiles, compatibility flags, constraints and level are not hardcoded', () => {
 assert.equal(initializationMIME(record('hvcC',[1,2,0x20,0,0,0,0xB0,0,0,0,0,0,153,0,0])), 'video/mp4; codecs="hvc1.2.4.L153.B0"');
 assert.equal(initializationMIME(record('hvcC',[1,1,0x60,0,0,0,0x90,0,0,0,0,0,120,0,0])), 'video/mp4; codecs="hvc1.1.6.L120.90"');
});
test('missing or truncated codec data is rejected', () => {
 assert.throws(() => initializationMIME(new Uint8Array()), /browser_cannot_decode_video/);
 assert.throws(() => initializationMIME(record('hvcC',[1,2,3])), /browser_cannot_decode_video/);
});
