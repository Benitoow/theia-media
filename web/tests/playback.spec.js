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
 if (await restart.isVisible()) await restart.click();
 await expect.poll(() => page.locator('video').evaluate(v => v.currentTime).catch(() => 0), { timeout: 35_000 }).toBeGreaterThan(2);
}
for (const name of ['Direct','Converted','Remux']) {
 test(`${name}: decoded frames, seek clock, pause and no runtime errors`, async ({ page }) => {
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  const movie=movies.find(m=>m.file_name.includes(name));expect(movie).toBeTruthy();
  await openPlayer(page, `/film/${movie.id}`);
  expect(await page.locator('video').evaluate(v=>v.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(20);
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
 expect(await page.locator('video').evaluate(v=>v.currentSrc)).toMatch(/^blob:/);
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
 let first=true;
 await page.route('**/info*',route=>{ if(first){first=false;return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'ffmpeg_unavailable'})});}return route.continue(); });
 const movie=movies.find(m=>m.file_name.includes('Direct'));
 await page.goto(`/film/${movie.id}`);await page.getByRole('button',{name:/^(Lire|Reprendre)/}).first().click();
 await page.getByRole('button',{name:'Réessayer',exact:true}).click();
 const restart=page.getByRole('button',{name:/du début/i});if(await restart.isVisible())await restart.click();
 await expect.poll(()=>page.locator('video').evaluate(v=>v.currentTime).catch(()=>0),{timeout:20000}).toBeGreaterThan(1);
});
