import { join } from 'node:path';
import { label,validateManifest } from './contract.mjs';

export const images={api:'ghcr.io/zitadel/zitadel:v4.17.1',login:'ghcr.io/zitadel/zitadel-login:v4.17.1',proxy:'traefik:v3.6.8',postgres:'postgres:17.2-alpine'};
export function composeConfiguration(m) {
  validateManifest(m);
  const labels={[label]:m.runId};
  const volume=key=>({type:'volume',source:key,target:({identity:'/var/lib/postgresql/data',commercial:'/var/lib/postgresql/data',login:'/login',setup:'/setup'})[key]});
  const service=(name,values)=>({container_name:`${m.project}-${name}`,restart:'no',labels,networks:['run'],...values});
  const port=(published,target)=>({host_ip:'127.0.0.1',published:String(published),target});
  const health={interval:'2s',timeout:'5s',retries:100,start_period:'10s'};
  const pg=(name,password,published)=>service(name,{image:images.postgres,environment:{POSTGRES_USER:'issue357',POSTGRES_PASSWORD:password,POSTGRES_DB:'issue357'},healthcheck:{...health,test:['CMD','pg_isready','-U','issue357','-d','issue357']},volumes:[volume(name==='identity-db'?'identity':'commercial')],...(published?{ports:[port(published,5432)]}:{})});
  return {name:m.project,services:{
    'identity-db':pg('identity-db',m.secrets.identityDatabase),
    'commercial-db':pg('commercial-db',m.secrets.commercialDatabase,m.ports.database),
    'zitadel-api':service('zitadel-api',{image:images.api,user:'0',command:['start-from-init','--masterkeyFromEnv'],environment:{
      ZITADEL_MASTERKEY:m.secrets.masterkey,ZITADEL_PORT:8080,ZITADEL_EXTERNALDOMAIN:'localhost',ZITADEL_EXTERNALPORT:m.ports.issuer,ZITADEL_EXTERNALSECURE:false,ZITADEL_TLS_ENABLED:false,
      ZITADEL_DATABASE_POSTGRES_DSN:`postgresql://issue357:${encodeURIComponent(m.secrets.identityDatabase)}@identity-db:5432/issue357?sslmode=disable`,
      ZITADEL_FIRSTINSTANCE_INSTANCENAME:m.project,ZITADEL_FIRSTINSTANCE_ORG_NAME:`${m.project}-bootstrap`,
      ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD:m.secrets.bootstrapPassword,ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED:false,
      ZITADEL_FIRSTINSTANCE_PATPATH:'/setup/bootstrap.pat',ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_USERNAME:'issue357-setup',ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_NAME:'Issue357 setup only',ZITADEL_FIRSTINSTANCE_ORG_MACHINE_PAT_EXPIRATIONDATE:new Date(Date.now()+86400000).toISOString(),
      ZITADEL_FIRSTINSTANCE_LOGINCLIENTPATPATH:'/login/login.pat',ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_USERNAME:'issue357-login',ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_NAME:'Issue357 official login',ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_PAT_EXPIRATIONDATE:new Date(Date.now()+86400000).toISOString(),
      ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED:true,ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_BASEURI:`${m.origins.issuer}/ui/v2/login/`,
      ZITADEL_OIDC_DEFAULTLOGINURLV2:`${m.origins.issuer}/ui/v2/login/login?authRequest=`,ZITADEL_OIDC_DEFAULTLOGOUTURLV2:`${m.origins.issuer}/ui/v2/login/logout?post_logout_redirect=`,
      ZITADEL_DEFAULTINSTANCE_OIDCSETTINGS_ACCESSTOKENLIFETIME:'120s',ZITADEL_DEFAULTINSTANCE_OIDCSETTINGS_IDTOKENLIFETIME:'120s',ZITADEL_DEFAULTINSTANCE_OIDCSETTINGS_REFRESHTOKENIDLEEXPIRATION:'20m',ZITADEL_DEFAULTINSTANCE_OIDCSETTINGS_REFRESHTOKENEXPIRATION:'1h',
      ZITADEL_LOG_LEVEL:'warn',ZITADEL_LOGSTORE_ACCESS_STDOUT_ENABLED:false,ZITADEL_TELEMETRY_ENABLED:false,ZITADEL_INSTRUMENTATION_TRACE_EXPORTER_TYPE:'none',
    },volumes:[volume('setup'),volume('login')],healthcheck:{...health,test:['CMD','/app/zitadel','ready']},depends_on:{'identity-db':{condition:'service_healthy'}}}),
    'zitadel-login':service('zitadel-login',{image:images.login,user:'0',environment:{ZITADEL_API_URL:'http://zitadel-api:8080',NEXT_PUBLIC_BASE_PATH:'/ui/v2/login',ZITADEL_SERVICE_USER_TOKEN_FILE:'/login/login.pat',CUSTOM_REQUEST_HEADERS:`Host:localhost:${m.ports.issuer},X-Forwarded-Proto:http`,OTEL_SDK_DISABLED:'true'},volumes:[{...volume('login'),read_only:true}],healthcheck:{...health,test:['CMD','node','/app/healthcheck.mjs','http://localhost:3000/ui/v2/login/healthy']},depends_on:{'zitadel-api':{condition:'service_healthy'}}}),
    proxy:service('proxy',{image:images.proxy,command:['--providers.file.filename=/etc/traefik/run.json','--entrypoints.web.address=:80','--accesslog=false','--log.level=WARN'],ports:[port(m.ports.issuer,80)],volumes:[{type:'bind',source:join(m.directory,'proxy.json'),target:'/etc/traefik/run.json',read_only:true}],depends_on:{'zitadel-api':{condition:'service_healthy'},'zitadel-login':{condition:'service_healthy'}}}),
  },networks:{run:{name:`${m.project}-network`,labels}},volumes:Object.fromEntries(['identity','commercial','login','setup'].map(key=>[key,{name:`${m.project}-${key}`,labels}]))};
}
export function proxyConfiguration() {
  return {http:{
    routers:{
      login:{entryPoints:['web'],rule:'PathPrefix(`/ui/v2/login`)',priority:100,service:'login'},
      api:{entryPoints:['web'],rule:'PathPrefix(`/`)',priority:1,service:'api'},
    },
    services:{
      login:{loadBalancer:{servers:[{url:'http://zitadel-login:3000'}]}},
      api:{loadBalancer:{servers:[{url:'h2c://zitadel-api:8080'}]}},
    },
  }};
}
