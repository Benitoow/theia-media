import { expect, test } from '@playwright/test';

let movies;
test.beforeAll(async ({ request }) => { movies = (await (await request.get('/api/library/movies')).json()).movies; });
test.beforeEach(async ({ page }) => {
 await page.addInitScript(() => { localStorage.setItem('theia.profile', '1'); localStorage.setItem('theia.locale', 'fr'); });
 // Preview generation is independent; a playback regression should not spend
 // its encoder budget generating thumbnails in the background.
 await page.route('**/preview*', route => route.abort());
});
async function openPlayer(page, path) {
 await page.goto(path);
 await page.getByRole('button', { name: /^(Lire|Reprendre)/ }).first().click();
 const restart = page.getByRole('button', { name: /du début/i });
 await expect.poll(async()=>await page.locator('video').count()>0||await restart.isVisible(),{timeout:10000}).toBe(true);
 if (await restart.isVisible()) await restart.click();
 await expect.poll(() => page.locator('video').evaluate(v => v.currentTime).catch(() => 0), { timeout: 35_000 }).toBeGreaterThan(2);
}
function mp4Box(type,payload=Buffer.alloc(0)) {
 const box=Buffer.alloc(8+payload.length);box.writeUInt32BE(box.length,0);box.write(type,4,4,'ascii');payload.copy(box,8);return box;
}
const unsupportedHEVCInitialization=()=>mp4Box('moov',mp4Box('hvcC',Buffer.from([1,2,0x20,0,0,0,0xB0,0,0,0,0,0,153,0,0])));
test('a measured file explains the plan for this browser before playback', async ({ page }) => {
	const movie = movies.find((item) => item.file_name.includes('Converted'));
	await page.goto(`/film/${movie.id}`);
	const panel = page.getByRole('region', { name: 'Lecture sur cet appareil' });
	await expect(panel).toBeVisible();
	const analyze = page.getByRole('button', { name: 'Analyser', exact: true });
	// The server fixture is shared by the browser projects. Chromium performs
	// the measurement; Edge must accept and explain that same persisted fact.
	if (await analyze.isVisible()) await analyze.click();
	await expect(panel).toContainText('Adapté par Theia', { timeout: 20_000 });
	await expect(panel).toContainText(/convertira/i);
});
for (const name of ['Direct','Converted','Remux']) {
 test(`${name}: decoded frames, seek clock, pause and no runtime errors`, async ({ page, browserName }) => {
  test.skip(browserName==='webkit'&&name==='Remux','Playwright WebKit on Windows exposes neither MSE nor ManagedMediaSource; its native fMP4 seek is not a Safari hardware acceptance test');
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  const movie=movies.find(m=>m.file_name.includes(name));expect(movie).toBeTruthy();
  await openPlayer(page, `/film/${movie.id}`);
  const picture=await page.locator('video').evaluate(v=>({width:v.videoWidth,height:v.videoHeight,frames:v.getVideoPlaybackQuality?.().totalVideoFrames??v.webkitDecodedFrameCount??0}));
  expect(picture.width).toBeGreaterThan(0);expect(picture.height).toBeGreaterThan(0);
  // Playwright WebKit advances and paints the video but exposes neither a
  // useful quality counter nor Safari's frame counter on Windows.
  if(browserName!=='webkit')expect(picture.frames).toBeGreaterThan(20);
  const slider=page.getByRole('slider',{name:'Position dans le film'});
  expect((await slider.boundingBox()).height).toBeGreaterThanOrEqual(44);
  await slider.focus();await page.keyboard.press('ArrowRight');
  await expect.poll(async()=>Number(await slider.getAttribute('aria-valuenow'))).toBeGreaterThan(11);
  const before=Number(await slider.getAttribute('aria-valuenow'));
  await expect.poll(async()=>Number(await slider.getAttribute('aria-valuenow'))).toBeGreaterThan(before+1);
  await page.keyboard.press('Space');await expect.poll(()=>page.locator('video').evaluate(v=>v.paused)).toBe(true);
  await page.keyboard.press('Escape');await expect(page.locator('video')).toHaveCount(0);
  expect(errors).toEqual([]);
 });
}
test('MPEG-2 episode has the same playable conversion path',async({page,request})=>{
 const series=(await(await request.get('/api/library/series')).json()).series;
 await page.goto(`/serie/${series[0].id}`);
 const episode=page.locator('a[href^="/episode/"]').first();await expect(episode).toBeVisible();
 const path=await episode.getAttribute('href');await openPlayer(page,path);
 const transport=await page.locator('video').evaluate(v=>({currentSrc:v.currentSrc,mse:Boolean(globalThis.MediaSource??globalThis.ManagedMediaSource)}));
 if(transport.mse)expect(transport.currentSrc).toMatch(/^blob:/);
 else expect(transport.currentSrc).toMatch(/\/remux\?/);
 await page.keyboard.press('Escape');
});
test('watch later persists and duration selection narrows the collection',async({page})=>{
 const movie=movies.find(m=>m.title.includes('Direct'));
 await page.goto(`/film/${movie.id}`);
 await page.getByRole('button',{name:'À voir plus tard',exact:true}).click();
 await expect(page.getByRole('button',{name:'Dans ma liste',exact:true})).toHaveAttribute('aria-pressed','true');
 await page.reload();await expect(page.getByRole('button',{name:'Dans ma liste',exact:true})).toBeVisible();
 await page.goto('/films?list=1');await expect(page.locator('.library-grid a')).toHaveCount(1);
 await page.goto('/films');await page.getByRole('button',{name:'90 minutes',exact:true}).click();
 await expect(page.locator('.library-grid a')).toHaveCount(1);
 await page.getByRole('button',{name:'Choisir pour moi'}).click();await expect(page).toHaveURL(new RegExp(`/film/${movie.id}$`));
 await page.getByRole('button',{name:'Dans ma liste',exact:true}).click();
});
test('audio selection survives seeks and text subtitles render',async({page})=>{
 const movie=movies.find(m=>m.file_name.includes('Remux'));
 await openPlayer(page,`/film/${movie.id}`);
 await page.mouse.move(500,700);
 await page.locator('.player-tracks-anchor > button').click();
 const lists=page.locator('.player-tracks .track-list');
 await lists.first().getByRole('button',{name:/Français/}).click();
 await expect.poll(()=>page.locator('video').getAttribute('data-stream-source')).toMatch(/audio=/);
 await page.mouse.move(500,700);await page.locator('.player-tracks-anchor > button').click();
 await page.locator('.player-tracks .track-list').last().getByRole('button',{name:/Français/}).click();
 await expect(page.locator('.player-subtitles')).toContainText('Theia playback guard');
 const source=await page.locator('video').getAttribute('data-stream-source');
 const slider=page.getByRole('slider',{name:'Position dans le film'});await slider.focus();await page.keyboard.press('End');
 const audio=new URL(source,'http://localhost').searchParams.get('audio');
 await expect.poll(async()=>new URL(await page.locator('video').getAttribute('data-stream-source'),'http://localhost').searchParams.get('audio')).toBe(audio);
 await expect.poll(async()=>Number(await slider.getAttribute('aria-valuenow'))).toBeGreaterThan(40);
 await page.keyboard.press('Escape');
});

test('a failed startup offers a working retry',async({page})=>{
 let failNextInfo=false;
 await page.route('**/info*',route=>{ if(failNextInfo){failNextInfo=false;return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'ffmpeg_unavailable'})});}return route.continue(); });
 const movie=movies.find(m=>m.file_name.includes('Direct'));
 await page.goto(`/film/${movie.id}`);
 await expect(page.getByRole('region',{name:'Lecture sur cet appareil'})).toBeVisible();
 failNextInfo=true;
 await page.getByRole('button',{name:/^(Lire|Reprendre)/}).first().click();
 await page.getByRole('button',{name:'Réessayer',exact:true}).click();
 const restart=page.getByRole('button',{name:/du début/i});
 await expect.poll(async()=>await page.locator('video').count()>0||await restart.isVisible(),{timeout:10000}).toBe(true);
 if(await restart.isVisible())await restart.click();
 await expect.poll(()=>page.locator('video').evaluate(v=>v.currentTime).catch(()=>0),{timeout:20000}).toBeGreaterThan(1);
});

test('MSE quota pressure retries the refused batch instead of failing playback',async({page,browserName})=>{
 test.skip(browserName!=='chromium','prototype quota injection is only reliable in Chromium/Edge; pure buffer tests cover the shared policy');
 await page.addInitScript(()=>{
  const append=SourceBuffer.prototype.appendBuffer;let calls=0,hit=false;
  window.__theiaQuotaHit=false;window.__theiaAppendAfterQuota=false;
  SourceBuffer.prototype.appendBuffer=function(bytes){
   calls++;if(hit)window.__theiaAppendAfterQuota=true;
   // Refuse a batch only after an earlier fragment is genuinely buffered.
   // This models rolling-buffer pressure without turning the empty startup
   // reserve into an artificial hard failure.
   const ranges=this.buffered;const clock=document.querySelector('video')?.currentTime??0;
   if(!hit&&ranges.length&&ranges.end(ranges.length-1)-clock>0.25){hit=true;window.__theiaQuotaHit=true;throw new DOMException('SourceBuffer is full','QuotaExceededError');}
   return append.call(this,bytes);
  };
 });
 const movie=movies.find(m=>m.file_name.includes('Remux'));
 await openPlayer(page,`/film/${movie.id}`);
 await expect.poll(()=>page.evaluate(()=>window.__theiaQuotaHit)).toBe(true);
 await expect.poll(()=>page.evaluate(()=>window.__theiaAppendAfterQuota)).toBe(true);
 await expect.poll(()=>page.locator('video').evaluate(v=>v.currentTime)).toBeGreaterThan(4);
 await expect(page.locator('.player-message')).toHaveCount(0);
});

test('a long pause releases a fragmented stream and resumes at its position',async({page,browserName})=>{
 test.skip(browserName==='webkit','Playwright WebKit on Windows has no MSE/ManagedMediaSource transport to exercise this fragmented-stream lifecycle');
 await page.addInitScript(()=>{
  const nativeSetTimeout=window.setTimeout.bind(window);
  window.setTimeout=(callback,delay,...args)=>nativeSetTimeout(callback,delay===120000?100:delay,...args);
 });
 const movie=movies.find(m=>m.file_name.includes('Remux'));
 await openPlayer(page,`/film/${movie.id}`);
 const before=await page.locator('video').evaluate(v=>v.currentTime);
 await page.keyboard.press('Space');
 await expect(page.locator('video')).toHaveCount(0,{timeout:5000});
 await page.keyboard.press('Space');
 await expect.poll(()=>page.locator('video').evaluate(v=>v.currentTime).catch(()=>0),{timeout:30000}).toBeGreaterThan(before);
 await page.keyboard.press('Escape');
});

test('a fresh info snapshot is reloaded when ffmpeg becomes ready during the first risky stream',async({page,browserName})=>{
 test.skip(browserName==='webkit','this fixture injects an MSE initialization segment; WebKit uses the native stream fallback here');
 await page.addInitScript(()=>{
  const supported=MediaSource.isTypeSupported.bind(MediaSource);
  window.__theiaMIMEs=[];
  Object.defineProperty(MediaSource,'isTypeSupported',{configurable:true,value:(mime)=>{window.__theiaMIMEs.push(mime);return mime.includes('hvc1')?false:supported(mime);}});
 });
 const diagnosticEvents=[];
 page.on('request',request=>{if(request.url().endsWith('/api/diagnostics/events'))diagnosticEvents.push(request.postDataJSON()?.event);});
 let infoRequests=0,injectPlayerInfo=false,firstRiskyStream=true;
 await page.route('**/info*',async route=>{
  const response=await route.fetch();const body=await response.json();infoRequests++;
  if(injectPlayerInfo){body.mode='remux';body.video_risky=true;body.video_codec='hevc';body.ffmpeg_ready=false;body.transcode={...body.transcode,available:false};}
  await route.fulfill({response,json:body});
 });
 await page.route(/\/remux\?/,route=>{
  const forced=new URL(route.request().url()).searchParams.get('video')==='transcode';
  if(firstRiskyStream&&!forced){firstRiskyStream=false;injectPlayerInfo=false;return route.fulfill({status:200,contentType:'video/mp4',body:unsupportedHEVCInitialization()});}
  return route.continue();
 });
 const movie=movies.find(m=>m.file_name.includes('Direct'));
 await page.goto(`/film/${movie.id}`);
 await expect(page.getByRole('region',{name:'Lecture sur cet appareil'})).toBeVisible();
 const beforePlayer=infoRequests;injectPlayerInfo=true;
 await page.getByRole('button',{name:/^(Lire|Reprendre)/}).first().click();
 await expect.poll(()=>firstRiskyStream,{timeout:10000}).toBe(false);
 await expect.poll(()=>page.evaluate(()=>window.__theiaMIMEs)).toContainEqual(expect.stringContaining('hvc1'));
 await expect.poll(()=>infoRequests,{timeout:10000}).toBeGreaterThanOrEqual(beforePlayer+2);
 await expect.poll(()=>diagnosticEvents,{timeout:10000}).toContain('quality_adapted');
 await expect.poll(()=>page.locator('video').getAttribute('data-stream-source'),{timeout:10000}).toContain('video=transcode');
 await expect.poll(()=>page.locator('video').evaluate(v=>v.currentTime).catch(()=>0),{timeout:25000}).toBeGreaterThan(1);
});
