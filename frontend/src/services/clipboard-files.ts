// A named file or an image with a source URL/HTML belongs in local storage.
// Only a raw clipboard image can be treated as a screenshot. On Windows the
// native file-drop format also protects real files named "image.png".
export async function clipboardFileImports(data:DataTransfer,hasNativeFiles:()=>Promise<boolean>){
 const files=Array.from(data.files)
 if(!files.length)for(const item of Array.from(data.items)){if(item.kind==='file'){const file=item.getAsFile();if(file)files.push(file)}}
 const html=data.getData('text/html'),uri=data.getData('text/uri-list'),text=data.getData('text/plain').trim()
 const candidate=files.length===1&&files[0]!.type.startsWith('image/')&&!html&&!uri&&!/^(?:https?:|file:)/i.test(text)&&/^(?:(?:image|clipboard|screenshot|pasted[-_ ]?image|截图|屏幕截图)(?:[-_ ]?\d+)?)?\.(?:png|jpe?g|webp|bmp)$/i.test(files[0]!.name||'.png')
 let screenshot=false
 if(candidate){try{screenshot=!await hasNativeFiles()}catch{/* Unknown clipboard origin stays local. */}}
 return files.map(file=>({file,screenshot}))
}
