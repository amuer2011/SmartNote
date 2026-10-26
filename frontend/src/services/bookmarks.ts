export function bookmarkLine(content:string,anchor:{line:number;text:string;before:string;after:string}){
 const lines=content.replace(/\r\n/g,'\n').split('\n')
 let best=-Infinity,index=Math.max(0,Math.min(anchor.line,lines.length-1)),found=false
 lines.forEach((text,i)=>{
  if(text!==anchor.text)return
  const score=(i>0&&lines[i-1]===anchor.before?4:0)+(i+1<lines.length&&lines[i+1]===anchor.after?4:0)-Math.abs(i-anchor.line)/Math.max(lines.length,1)
  if(score>best){best=score;index=i;found=true}
 })
 return {line:index,found}
}
