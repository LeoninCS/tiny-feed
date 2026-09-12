const fs = require('node:fs');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const playwright = require(process.env.QA_PLAYWRIGHT_PATH || 'playwright');
const origin = process.env.QA_BASE_URL || 'http://127.0.0.1:18082';
assert(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(origin).hostname), 'Tests must use an isolated local server');
const uploadOrigin = process.env.QA_UPLOAD_ORIGIN || origin;
assert(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(uploadOrigin).hostname), 'Upload tests must use an isolated local server');
assert(process.env.QA_COMPOSE_FILE && process.env.QA_VIDEO_FILE, 'QA_COMPOSE_FILE and QA_VIDEO_FILE are required');
const secret = JSON.parse(fs.readFileSync(process.env.QA_COMPOSE_FILE)).services.backend.environment.JWT_SECRET;
const clip = fs.readFileSync(process.env.QA_VIDEO_FILE);
const cover = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/lQAAAABJRU5ErkJggg==', 'base64');
function expiredToken(account) {
 const head = Buffer.from(JSON.stringify({alg:'HS256',typ:'JWT'})).toString('base64url');
 const body = Buffer.from(JSON.stringify({account_id:account.account_id,username:account.username,exp:1,iat:0,nbf:0})).toString('base64url');
 const signature = crypto.createHmac('sha256',secret).update(head+'.'+body).digest('base64url');
 return head+'.'+body+'.'+signature;
}
(async()=>{
 const kind = process.env.QA_BROWSER || 'chromium';
 const browser = await playwright[kind].launch({headless:true});
 const context = await browser.newContext({viewport:{width:393,height:852},isMobile:true,hasTouch:true});
 const page = await context.newPage();
 const requests = {video:0,cover:0,publish:0,refresh:[]};
 const uploadHosts = new Set();
 const mediaHosts = new Set();
 page.on('request',r=>{if(r.url().endsWith('/video/uploadVideo'))requests.video++;if(r.url().endsWith('/video/uploadCover'))requests.cover++;if(r.url().endsWith('/video/publish'))requests.publish++;});
 page.on('request',r=>{if(/\/video\/upload(Video|Cover)$/.test(r.url()) && r.method()==='POST')uploadHosts.add(new URL(r.url()).origin);});
 page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/static/'))mediaHosts.add(new URL(r.url()).origin);});
 page.on('response',r=>{if(r.url().endsWith('/account/refresh'))requests.refresh.push(r.status());});
 const credentials={username:'local_qa_'+crypto.randomBytes(7).toString('hex'),password:crypto.randomBytes(20).toString('hex')};
 try {
  if (uploadOrigin !== origin) {
   const health = await context.request.get(uploadOrigin+'/healthz');
   assert.equal(health.status(),200);assert.equal(health.headers()['x-upload-transport'],'direct');
   const preflight = await context.request.fetch(uploadOrigin+'/api/video/uploadVideo',{method:'OPTIONS',headers:{Origin:origin,'Access-Control-Request-Method':'POST','Access-Control-Request-Headers':'authorization,content-type'}});
   assert.equal(preflight.status(),204);assert.equal(preflight.headers()['access-control-allow-origin'],origin);
   const unauthorized = await context.request.post(uploadOrigin+'/api/video/uploadVideo',{headers:{Origin:origin}});
   assert.equal(unauthorized.status(),401);assert.equal(unauthorized.headers()['access-control-allow-origin'],origin);
   const forbidden = await context.request.post(uploadOrigin+'/api/video/uploadVideo',{headers:{Origin:'https://untrusted.example'}});
   assert.equal(forbidden.status(),403);assert.equal(forbidden.headers()['access-control-allow-origin'],undefined);
   const otherRoute = await context.request.post(uploadOrigin+'/api/account/login',{data:{}});
   assert.equal(otherRoute.status(),404);
   console.log(kind,'PASS direct gateway health, CORS, authentication and route isolation');
  }
  const register=await context.request.post(origin+'/api/account/register',{data:credentials});assert.equal(register.status(),200);
  const login=await context.request.post(origin+'/api/account/login',{data:credentials});assert.equal(login.status(),200);const account=await login.json();
  await page.goto(origin+'/account');
  await page.evaluate(a=>{localStorage.setItem('access_token',a.token);localStorage.setItem('refresh_token',a.refresh_token);},account);
  async function select(title,mime='video/mp4',coverMime='image/png'){
   await page.goto(origin+'/video');
   await page.locator('input[type="text"]').fill(title);
   await page.locator('input[type="file"]').nth(0).setInputFiles({name:title+'.mp4',mimeType:mime,buffer:clip});
   await page.locator('input[type="file"]').nth(1).setInputFiles({name:title+'.png',mimeType:coverMime,buffer:cover});
  }
  async function publish(){
   const sent=page.waitForResponse(r=>r.url().endsWith('/video/publish'));
   await page.locator('.big-btn').click();
   const result=await sent;assert.equal(result.status(),200);const video=await result.json();
   await page.getByRole('link',{name:'去播放'}).waitFor();
   const stored=await context.request.get(origin+video.play_url);
   assert.equal(stored.status(),200);assert.deepEqual(await stored.body(),clip);
   return video;
  }
  await select('普通手机视频');const firstVideo=await publish();console.log(kind,'PASS real video + cover + publish + byte-for-byte stored file');
  if (process.env.QA_VERIFY_MEDIA === '1') {
   const health=await context.request.get(origin+'/healthz',{maxRedirects:0});
   assert.equal(health.status(),200);assert.equal(health.headers()['x-media-transport'],'direct');
   const mediaUrl=new URL(firstVideo.play_url,origin).href;
   const head=await context.request.head(mediaUrl);
   assert.equal(head.status(),200);assert.equal(Number(head.headers()['content-length']),clip.length);
   assert.equal(head.headers()['accept-ranges'],'bytes');assert.equal(head.headers()['x-media-transport'],'direct');
   const part=await context.request.get(mediaUrl,{headers:{Range:'bytes=1024-2047','If-Range':head.headers()['last-modified']}});
   assert.equal(part.status(),206);assert.equal(part.headers()['content-range'],`bytes 1024-2047/${clip.length}`);
   assert.deepEqual(await part.body(),clip.subarray(1024,2048));
   const tail=await context.request.get(mediaUrl,{headers:{Range:'bytes=-1024'}});
   assert.equal(tail.status(),206);assert.deepEqual(await tail.body(),clip.subarray(-1024));
   const invalid=await context.request.get(mediaUrl,{headers:{Range:`bytes=${clip.length}-`}});
   assert.equal(invalid.status(),416);
   await page.getByRole('link',{name:'去播放'}).click();
   await page.waitForFunction(()=>{const v=document.querySelector('video');return v?.readyState>=2});
   const videoEl=page.locator('video');
   assert.equal(await videoEl.evaluate(v=>v.muted),false,'default playback must remain audible');
   assert.equal(new URL(await videoEl.evaluate(v=>v.currentSrc)).origin,origin);
   if(await videoEl.evaluate(v=>v.paused)){
    const hint=page.getByRole('button',{name:'点击有声播放'});
    if(await hint.isVisible())await hint.click();else await videoEl.click();
   }
   await page.waitForFunction(()=>document.querySelector('video')?.currentTime>0.2);
   await videoEl.evaluate(v=>{v.currentTime=Math.min(3,v.duration/2)});
   await page.waitForFunction(()=>{const v=document.querySelector('video');return !v.seeking && v.currentTime>=Math.min(3,v.duration/2)});
   await page.goto(origin+'/');
   await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2);
   assert.equal(await page.locator('video').first().evaluate(v=>v.muted),false);
   assert.deepEqual([...mediaHosts],[origin],'feed, detail, video and covers must stay on the direct HTTP origin');
   console.log(kind,'PASS direct HTTP health, HEAD, Range, If-Range, suffix, invalid range, audible playback and seek');
  }
  for(let i=0;i<2;i++){
   await page.evaluate(token=>localStorage.setItem('access_token',token),expiredToken(account));
   await select('续期测试'+i);
   const previousRefresh=await page.evaluate(()=>localStorage.getItem('refresh_token'));
   await publish();
   const nextRefresh=await page.evaluate(()=>localStorage.getItem('refresh_token'));
   assert.notEqual(nextRefresh,previousRefresh,'rotated refresh token must be persisted');
  }
  assert.deepEqual(requests.refresh,[200,200]);
  assert.equal(requests.video,3,'expired access token should refresh before sending video');
  console.log(kind,'PASS two consecutive real refresh rotations, no double video upload');
  await select('手机通用MIME','application/octet-stream','application/octet-stream');await publish();
  await select('手机缺失MIME','','');await publish();console.log(kind,'PASS generic and empty MIME video + cover');
  await select('封面断网重试');
  let failCover=true;
  await page.route(uploadOrigin+'/api/video/uploadCover',async route=>{if(failCover){failCover=false;await route.fulfill({status:503,headers:{'Access-Control-Allow-Origin':origin},json:{error:'模拟封面连接暂时不可用'}});}else await route.continue();});
  await page.locator('.big-btn').click();
  await page.getByRole('alert').filter({hasText:'上传封面失败'}).waitFor();
  const sentVideos=requests.video;
  await publish();assert.equal(requests.video,sentVideos,'cover retry must not resend the video');
  await page.unroute(uploadOrigin+'/api/video/uploadCover');console.log(kind,'PASS interrupted cover retry reuses uploaded video');
  await page.evaluate(token=>localStorage.setItem('access_token',token),expiredToken(account));
  await select('续期临时失败');
  await page.route(origin+'/api/account/refresh',route=>route.fulfill({status:503,json:{error:'模拟临时故障'}}));
  const beforeFailure=requests.video;
  await page.locator('.big-btn').click();
  await page.getByRole('alert').filter({hasText:'登录状态验证暂时失败'}).waitFor();
  assert.equal(await page.evaluate(()=>!!localStorage.getItem('refresh_token')),true);
  assert.equal(requests.video,beforeFailure);
  await page.unroute(origin+'/api/account/refresh');
  await publish();console.log(kind,'PASS transient refresh failure preserves login and selection, retry succeeds');
  await page.evaluate(token=>{localStorage.setItem('access_token',token);localStorage.setItem('refresh_token','revoked-test-token');},expiredToken(account));
  await select('失效登录拦截');
  const beforeRevoked=requests.video;
  await page.locator('.big-btn').click();
  await page.getByRole('alert').filter({hasText:'登录已失效'}).waitFor();
  assert.equal(requests.video,beforeRevoked);
  await page.getByRole('link',{name:'重新登录',exact:true}).waitFor();
  console.log(kind,'PASS revoked login rejected before sending file, visible login action');
  assert.deepEqual([...uploadHosts],[new URL(uploadOrigin).origin], 'All file requests must use the configured upload origin');
  if (process.env.QA_ARTIFACT_DIR) {
   fs.mkdirSync(process.env.QA_ARTIFACT_DIR, {recursive:true});
   await page.screenshot({path:require('node:path').join(process.env.QA_ARTIFACT_DIR, kind+'-regression.png')});
  }
 } finally {await context.close();await browser.close();}
})().catch(e=>{console.error(e.message);process.exitCode=1});
