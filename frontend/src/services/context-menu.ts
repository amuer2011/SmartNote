// Keep the pointer outside the menu, including when it opens near screen edges.
export function contextMenuPosition(x:number,y:number,width:number,height:number){
 const gap=10,edge=8,vw=window.innerWidth,vh=window.innerHeight
 const left=x+gap+width<=vw-edge?x+gap:x-width-gap
 const top=y+gap+height<=vh-edge?y+gap:y-height-gap
 return {x:Math.max(edge,Math.min(left,vw-width-edge)),y:Math.max(edge,Math.min(top,vh-height-edge))}
}
