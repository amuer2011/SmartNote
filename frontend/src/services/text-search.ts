export interface TextMatch {start:number;end:number;text:string;replacement:string}
export function textMatches(text:string,query:string,regex=false,caseSensitive=false,replacement=''):TextMatch[]{
 if(!query)return []
 const pattern=regex?query:query.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')
 const re=new RegExp(pattern,caseSensitive?'gm':'gmi'),matches:TextMatch[]=[]
 for(const match of text.matchAll(re)){
  const start=match.index!,end=start+match[0].length
  const expanded=regex?replacement.replace(/\$(\$|&|`|'|\d{1,2}|<[^>]+>)/g,(token,key:string)=>{
   if(key==='$')return '$';if(key==='&')return match[0];if(key==='`')return text.slice(0,start);if(key==="'")return text.slice(end)
   if(key.startsWith('<'))return match.groups?.[key.slice(1,-1)]??token
   const n=Number(key);if(n>0&&n<match.length)return match[n]??''
   if(key.length===2&&Number(key[0])>0&&Number(key[0])<match.length)return (match[Number(key[0])]??'')+key[1]
   return token
  }):replacement
  matches.push({start,end,text:match[0],replacement:expanded})
 }
 return matches
}
export function applyMatches(text:string,matches:TextMatch[]){let output='',from=0;for(const match of matches){output+=text.slice(from,match.start)+match.replacement;from=match.end}return output+text.slice(from)}
