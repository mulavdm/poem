"use strict";

const assetVersion = new URL(self.location.href).searchParams.get("v") || "";
const assetURL = path => `${path}?v=${encodeURIComponent(assetVersion)}`;
importScripts(assetURL("/__poem/wasm_exec.js"));

let canvas, gl, config, camera, ratio = 1, generation = 0, loadController;
let resources = new Map(), draws = [], program, uniforms = {};

const vertexShader = `#version 300 es
precision highp float;
layout(location=0) in vec2 aWorld;
layout(location=1) in vec4 aColor;
layout(location=2) in vec2 aUV;
layout(location=3) in vec2 aOffset;
layout(location=4) in float aWidth;
layout(location=5) in float aElevation;
uniform vec2 uCenter;
uniform vec2 uViewport;
uniform float uWorldPixels;
uniform float uBearing;
uniform float uPitchCos;
uniform float uPitchSin;
uniform float uCameraDistance;
uniform float uRatio;
uniform float uMetersToPixels;
uniform bool uPoints;
out vec4 vColor;
out vec2 vUV;
void main(){
  float dx=mod(aWorld.x-uCenter.x+1.5,1.0)-0.5;
  float dy=aWorld.y-uCenter.y;
  float cs=cos(uBearing), sn=sin(uBearing);
  float localX=(dx*cs-dy*sn)*uWorldPixels;
  float localY=(dx*sn+dy*cs)*uWorldPixels;
  float localZ=aElevation*uMetersToPixels;
  float projectedY=localY*uPitchCos-localZ*uPitchSin;
  float depth=localY*uPitchSin+localZ*uPitchCos;
  float perspective=uCameraDistance/max(uCameraDistance*.05,uCameraDistance-depth);
  vec2 pixel=uViewport*0.5+vec2(localX,projectedY)*perspective+aOffset*uRatio;
  vec2 clip=vec2(pixel.x/uViewport.x*2.0-1.0,1.0-pixel.y/uViewport.y*2.0);
  gl_Position=vec4(clip,clamp(-depth/uCameraDistance,-.99,.99),1.0);
  gl_PointSize=uPoints?max(3.0,aWidth*uRatio*2.0):1.0;
  vColor=aColor;vUV=aUV;
}`;
const fragmentShader = `#version 300 es
precision mediump float;
in vec4 vColor;in vec2 vUV;
uniform sampler2D uTexture;
uniform bool uTextured;uniform bool uPoints;uniform float uOpacity;
out vec4 color;
void main(){
  if(uPoints){vec2 p=gl_PointCoord-vec2(.5);if(dot(p,p)>.25)discard;}
  float alpha=uTextured?texture(uTexture,vUV).r:1.0;
  color=vec4(vColor.rgb,vColor.a*uOpacity*alpha);
}`;

const compile = (kind, source) => {
  const shader=gl.createShader(kind);gl.shaderSource(shader,source);gl.compileShader(shader);
  if(!gl.getShaderParameter(shader,gl.COMPILE_STATUS))throw new Error(gl.getShaderInfoLog(shader));
  return shader;
};
const initializeGL = () => {
  gl=canvas.getContext("webgl2",{alpha:true,antialias:true,premultipliedAlpha:true});
  if(!gl)throw new Error("WebGL2 unavailable");
  program=gl.createProgram();gl.attachShader(program,compile(gl.VERTEX_SHADER,vertexShader));gl.attachShader(program,compile(gl.FRAGMENT_SHADER,fragmentShader));gl.linkProgram(program);
  if(!gl.getProgramParameter(program,gl.LINK_STATUS))throw new Error(gl.getProgramInfoLog(program));
  for(const name of ["uCenter","uViewport","uWorldPixels","uBearing","uPitchCos","uPitchSin","uCameraDistance","uRatio","uMetersToPixels","uPoints","uTexture","uTextured","uOpacity"])uniforms[name]=gl.getUniformLocation(program,name);
  gl.enable(gl.BLEND);gl.blendFunc(gl.SRC_ALPHA,gl.ONE_MINUS_SRC_ALPHA);gl.useProgram(program);gl.uniform1i(uniforms.uTexture,0);
};
const startWASM = async () => {
  const go=new Go();
  const response=await fetch(assetURL("/__poem/cartography.wasm"),{credentials:"same-origin"});
  if(!response.ok)throw new Error(`WASM HTTP ${response.status}`);
  const result=await WebAssembly.instantiateStreaming(response,go.importObject);
  go.run(result.instance);
  for(let attempt=0;attempt<500&&!self.poemCartographyReady;attempt++)await new Promise(resolve=>setTimeout(resolve,10));
  if(!self.poemCartographyReady)throw new Error("cartography WASM startup timed out");
};
const cameraInput = value => ({Latitude:value.latitude,Longitude:value.longitude,Zoom:value.zoom,Bearing:value.bearing,Pitch:value.pitch,Width:Math.max(1,Math.round(config.width)),Height:Math.max(1,Math.round(config.height))});
const decodeBase64 = value => {
  const binary=atob(value), bytes=new Uint8Array(binary.length);
  for(let index=0;index<binary.length;index++)bytes[index]=binary.charCodeAt(index);
  return bytes;
};
const resourceURL = (tile, kind) => {
	const query=new URLSearchParams({provider:config.source.ProviderID,source:config.source.ID,snapshot:config.source.Snapshot||"",kind:String(kind),z:String(tile.Z),x:String(tile.X),y:String(tile.Y)});
  return `/__poem/map-resource?${query}`;
};
const fetchTiles = async (tiles, signal) => {
	const output=new Array(tiles.length);let next=0,totalBytes=0;
	const encodedBody=async response=>{const body=new Uint8Array(await response.arrayBuffer());totalBytes+=body.length;if(totalBytes>128*1024*1024)throw new Error("map resource byte budget exceeded");let binary="";for(let offset=0;offset<body.length;offset+=0x8000)binary+=String.fromCharCode(...body.subarray(offset,offset+0x8000));return btoa(binary);};
	const worker=async()=>{for(;;){const index=next++;if(index>=tiles.length)return;const tile=tiles[index];try{const response=await fetch(resourceURL(tile,0),{credentials:"same-origin",signal});if(!response.ok)continue;const result={z:tile.Z,x:tile.X,y:tile.Y,bytes:await encodedBody(response)};if(config.source.Elevation){const elevation=await fetch(resourceURL(tile,1),{credentials:"same-origin",signal});if(elevation.ok)result.elevation=await encodedBody(elevation);}output[index]=result;}catch(error){if(signal.aborted)throw error;}}};
  await Promise.all(Array.from({length:Math.min(8,tiles.length)},worker));return output.filter(Boolean);
};
const releaseResources = () => {for(const resource of resources.values()){if(resource.buffer)gl.deleteBuffer(resource.buffer);if(resource.texture)gl.deleteTexture(resource.texture);}resources.clear();};
const acceptScene = scene => {
  const nextResources=new Map();
  for(const item of scene.resources){const existing=resources.get(item.hash);if(existing){nextResources.set(item.hash,existing);continue;}const bytes=decodeBase64(item.bytes);const resource={...item,bytes};if(item.type===0||item.type===1){resource.buffer=gl.createBuffer();gl.bindBuffer(item.type===0?gl.ARRAY_BUFFER:gl.ELEMENT_ARRAY_BUFFER,resource.buffer);gl.bufferData(item.type===0?gl.ARRAY_BUFFER:gl.ELEMENT_ARRAY_BUFFER,bytes,gl.STATIC_DRAW);}else{resource.texture=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D,resource.texture);gl.pixelStorei(gl.UNPACK_ALIGNMENT,1);if(item.type===2)gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA8,item.width,item.height,0,gl.RGBA,gl.UNSIGNED_BYTE,bytes);else gl.texImage2D(gl.TEXTURE_2D,0,gl.R8,item.width,item.height,0,gl.RED,gl.UNSIGNED_BYTE,bytes);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.LINEAR);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.LINEAR);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);}nextResources.set(item.hash,resource);}
  for(const [hash,resource] of resources){if(nextResources.has(hash))continue;if(resource.buffer)gl.deleteBuffer(resource.buffer);if(resource.texture)gl.deleteTexture(resource.texture);}
  resources=nextResources;draws=scene.draws;render();
};
const loadScene = async () => {
  if(loadController)loadController.abort();const controller=new AbortController();loadController=controller;const current=++generation;
  try{const tileZoom=Math.max(0,Math.min(22,Math.floor(Math.min(camera.zoom,config.source.MaxZoom))));const covered=JSON.parse(self.poemCartographyCover(JSON.stringify({camera:cameraInput(camera),maximum:config.maximum||48,tile_zoom:tileZoom})));if(covered.error)throw new Error(covered.error);const tiles=await fetchTiles(covered,controller.signal);if(controller.signal.aborted||current!==generation)return;const built=JSON.parse(self.poemCartographyBuild(JSON.stringify({viewport_id:config.viewportID,generation:current,camera:cameraInput(camera),tiles,features:config.features,palette:config.palette,building_heights:config.buildingHeights,locale:config.locale,poi_categories:config.poiFilters,label_density:config.labelDensity,label_maximum:config.labelMaximum,terrain_scale:config.terrainScale,terrain_dimension:config.terrainDimension})));if(built.error)throw new Error(built.error);if(controller.signal.aborted||current!==generation)return;acceptScene(built);postMessage({type:"loaded",generation:current,picks:built.picks||[]});}catch(error){if(!controller.signal.aborted)postMessage({type:"failure",message:String(error&&error.message||error)});}
};
const mercator = value => {const lat=Math.max(-85.05112878,Math.min(85.05112878,value.latitude))*Math.PI/180;return[(value.longitude+180)/360,.5-Math.log((1+Math.sin(lat))/(1-Math.sin(lat)))/(4*Math.PI)];};
const render = () => {
  if(!gl)return;gl.viewport(0,0,canvas.width,canvas.height);gl.clearColor(0,0,0,0);gl.clearDepth(1);gl.clear(gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT);gl.useProgram(program);
  const center=mercator(camera),pitch=camera.pitch*Math.PI/180,worldPixels=512*Math.pow(2,camera.zoom)*ratio,cameraDistance=canvas.height*.5/Math.tan(Math.PI/8);gl.uniform2f(uniforms.uCenter,center[0],center[1]);gl.uniform2f(uniforms.uViewport,canvas.width,canvas.height);gl.uniform1f(uniforms.uWorldPixels,worldPixels);gl.uniform1f(uniforms.uBearing,-camera.bearing*Math.PI/180);gl.uniform1f(uniforms.uPitchCos,Math.cos(pitch));gl.uniform1f(uniforms.uPitchSin,Math.sin(pitch));gl.uniform1f(uniforms.uCameraDistance,cameraDistance);gl.uniform1f(uniforms.uRatio,ratio);gl.uniform1f(uniforms.uMetersToPixels,worldPixels/(40075016.68557849*Math.max(.01,Math.cos(camera.latitude*Math.PI/180))));
  for(const draw of draws){const vertex=resources.get(draw.vertex_hash),index=resources.get(draw.index_hash);if(!vertex||!index)continue;if(draw.depth_test)gl.enable(gl.DEPTH_TEST);else gl.disable(gl.DEPTH_TEST);gl.bindBuffer(gl.ARRAY_BUFFER,vertex.buffer);gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER,index.buffer);const stride=vertex.stride;gl.enableVertexAttribArray(0);gl.vertexAttribPointer(0,2,gl.FLOAT,false,stride,0);gl.enableVertexAttribArray(1);gl.vertexAttribPointer(1,4,gl.UNSIGNED_BYTE,true,stride,12);gl.enableVertexAttribArray(4);gl.vertexAttribPointer(4,1,gl.FLOAT,false,stride,16);gl.enableVertexAttribArray(5);gl.vertexAttribPointer(5,1,gl.FLOAT,false,stride,8);if(stride>=44){gl.enableVertexAttribArray(2);gl.vertexAttribPointer(2,2,gl.FLOAT,false,stride,28);gl.enableVertexAttribArray(3);gl.vertexAttribPointer(3,2,gl.FLOAT,false,stride,36);}else{gl.disableVertexAttribArray(2);gl.vertexAttrib2f(2,0,0);gl.disableVertexAttribArray(3);gl.vertexAttrib2f(3,0,0);}const textured=draw.texture_hash&&resources.get(draw.texture_hash);gl.uniform1i(uniforms.uTextured,textured?1:0);if(textured){gl.activeTexture(gl.TEXTURE0);gl.bindTexture(gl.TEXTURE_2D,textured.texture);}const points=draw.primitive===2;gl.uniform1i(uniforms.uPoints,points?1:0);gl.uniform1f(uniforms.uOpacity,draw.opacity);const mode=draw.primitive===0?gl.TRIANGLES:draw.primitive===1?gl.LINES:gl.POINTS;gl.drawElements(mode,draw.count,gl.UNSIGNED_INT,draw.first*4);}
};
self.onmessage = async event => {
  try{const message=event.data;if(message.type==="init"){canvas=message.canvas;config=message.config;camera=config.camera;ratio=message.ratio||1;canvas.width=message.width;canvas.height=message.height;initializeGL();await startWASM();await loadScene();postMessage({type:"ready"});}else if(message.type==="camera"){camera=message.camera;render();}else if(message.type==="settle"){camera=message.camera;await loadScene();}else if(message.type==="resize"){ratio=message.ratio||1;canvas.width=message.width;canvas.height=message.height;config.width=message.logicalWidth;config.height=message.logicalHeight;render();}else if(message.type==="dispose"){if(loadController)loadController.abort();releaseResources();close();}}catch(error){postMessage({type:"failure",message:String(error&&error.message||error)});}
};
