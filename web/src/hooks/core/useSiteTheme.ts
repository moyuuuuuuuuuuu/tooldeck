import {ref} from 'vue'
import {useTheme} from './useTheme'
import {SystemThemeEnum} from '@/enums/appEnum'
export type SiteTheme = 'light' | 'dark'
export const siteTheme=ref<SiteTheme>('light')
export function setSiteTheme(mode:SiteTheme){siteTheme.value=mode;document.dispatchEvent(new Event('tooldeck-theme-change'))}
export function initializeSiteTheme(){
 const {setSystemTheme}=useTheme()
 const hour=new Date().getHours()
 siteTheme.value=hour>=7&&hour<19?'light':'dark'
 const apply=()=>{setSystemTheme(siteTheme.value==='dark'?SystemThemeEnum.DARK:SystemThemeEnum.LIGHT);document.documentElement.style.colorScheme=siteTheme.value}
 apply()
 document.addEventListener('tooldeck-theme-change',apply)
 return ()=>document.removeEventListener('tooldeck-theme-change',apply)
}
