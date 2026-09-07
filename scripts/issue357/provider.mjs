import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {join} from 'node:path';
import {json,save} from './io.mjs';
import {secret,validateManifest} from './contract.mjs';

export async function provider(m,path,body,method='POST') {
 validateManifest(m);assert.ok(path.startsWith('/')&&!path.startsWith('//'),'INVALID_PATH');
 const token=(await readFile(join(m.directory,'bootstrap.pat'),'utf8')).trim();
 let response;
 try {response=await fetch(m.origins.issuer+path,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json','Connect-Protocol-Version':'1'},body:body===undefined?undefined:JSON.stringify(body),redirect:'error',signal:AbortSignal.timeout(30000)})}
 catch{throw new Error(method==='GET'?'DEPENDENCY_UNAVAILABLE':'MUTATION_OUTCOME_UNKNOWN')}
 let text='',size=0;
 for await(const chunk of response.body){size+=chunk.length;assert.ok(size<=1024*1024,'INVALID_PROVIDER_RESPONSE');text+=Buffer.from(chunk).toString('utf8')}
 if(!response.ok)throw new Error(`PROVIDER_HTTP_${response.status}:${path}`);
 return text?JSON.parse(text):{};
}
export async function createSubjects(m) {
 const me=await provider(m,'/auth/v1/users/me',undefined,'GET');assert.ok(me.user?.id,'INVALID_PROVIDER_RESPONSE');m.bootstrapUserId=me.user.id;
 // GetMyInstance's pinned admin.proto route (GetIAM belongs to Management).
 const instance=await provider(m,'/admin/v1/instances/me',undefined,'GET');m.instanceId=instance.instance?.id;
 assert.ok(m.instanceId,'INVALID_INSTANCE');await save(m);
 for(const key of ['A','B','C','Empty','D']) {
  const name=`${m.project}-${key}`;const r=await provider(m,'/v2/organizations',{name});assert.ok(r.organizationId,'INVALID_PROVIDER_RESPONSE');m.organizations[key]={id:r.organizationId,name};await save(m);
 }
 for(const key of ['admin','viewer','no-org']) {
  const username=`${key}.${m.runId.slice(0,8)}@example.test`,password=secret();
  const r=await provider(m,'/v2/users/human',{username,organization:{orgId:m.organizations.A.id},profile:{givenName:'Issue357',familyName:key,displayName:`Issue357 ${key}`,preferredLanguage:'en'},email:{email:username,isVerified:true},password:{password,changeRequired:false}});
  assert.ok(r.userId,'INVALID_PROVIDER_RESPONSE');const credentialFile=join(m.directory,`${key}.credentials.json`);
  await json(credentialFile,{username,password});m.users[key]={id:r.userId,homeOrganizationId:m.organizations.A.id,credentialFile};await save(m);
 }
}
export async function grantSubjects(m) {
 for(const key of ['B','C','Empty']) {
  const organizationId=m.organizations[key].id;
  await provider(m,'/zitadel.project.v2.ProjectService/CreateProjectGrant',{projectId:m.projectId,grantedOrganizationId:organizationId,roleKeys:['listingkit_admin','listingkit_viewer']});
  for(const user of ['admin','viewer']) {
   const r=await provider(m,'/zitadel.authorization.v2.AuthorizationService/CreateAuthorization',{userId:m.users[user].id,projectId:m.projectId,organizationId,roleKeys:[user==='admin'?'listingkit_admin':'listingkit_viewer']});
   assert.ok(r.id,'INVALID_PROVIDER_RESPONSE');m.authorizations[`${user}:${key}`]=r.id;await save(m);
  }
 }
}
export async function authorizationControl(m,user,org,action) {
 assert.ok(['admin','viewer'].includes(user)&&['B','C','Empty'].includes(org),'INVALID_CONTROL');
 assert.ok(['revoke','restore'].includes(action),'INVALID_CONTROL');
 const id=m.authorizations[`${user}:${org}`];assert.ok(id,'INVALID_CONTROL');
 async function readOwned(){
  const result=await provider(m,'/zitadel.authorization.v2.AuthorizationService/ListAuthorizations',{pagination:{limit:100},filters:[{inUserIds:{ids:[m.users[user].id]}},{projectId:{id:m.projectId}},{organizationId:{id:m.organizations[org].id}}]});
  const found=result.authorizations?.find(a=>a.id===id);assert.ok(found&&found.user.id===m.users[user].id&&found.project.id===m.projectId&&found.organization.id===m.organizations[org].id,'OWNERSHIP_MISMATCH');return found;
 }
 const found=await readOwned();
 const target=action==='revoke'?'STATE_INACTIVE':'STATE_ACTIVE';
 if(found.state!==target) await provider(m,`/zitadel.authorization.v2.AuthorizationService/${action==='revoke'?'DeactivateAuthorization':'ActivateAuthorization'}`,{id});
 assert.equal((await readOwned()).state,target,'CONTROL_POSTCONDITION_FAILED');
 await json(join(m.directory,'control.json'),{action,user,organization:org,id,at:new Date().toISOString()});
}
