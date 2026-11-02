#!/usr/bin/env python3
"""Synchronize scoped API descriptions from the UI catalog; retain legacy path routes."""
import copy
import json
from pathlib import Path
root = Path(__file__).resolve().parents[1]
path = root / 'photoly_server/openapi.json'
spec = json.loads(path.read_text())
endpoints = json.loads((root / 'photoly_frontend_vue/src/lib/credentialEndpoints.json').read_text())
schemas = spec['components']['schemas']
spec['components']['securitySchemes']['ApiCredential'] = {'type': 'http', 'scheme': 'bearer', 'description': 'Scoped C/R/D API token, not a login or share token.'}
def nullable(t): return {'type': [t, 'null']}
def envelope(t): return {'type': 'object', 'required': ['msgCode','t'], 'properties': {'msgCode': {'type':'integer'}, 't':t}}
def response(schema, description='Success'): return {'description': description, 'content': {'application/json': {'schema': schema}}}
def param(name, where='query', schema=None): return {'name':name,'in':where,'required':where=='path','schema':schema or {'type':'string'}}
media = {'type':'object','required':['photoId','uuid','name','format','nsId','contentVersion','animated','live','categories'],'properties':{'photoId':{'type':'integer'},'uuid':{'type':'string'},'name':{'type':'string'},'format':{'type':'string'},'nsId':nullable('integer'),'contentVersion':{'type':'integer'},'animated':{'type':'boolean'},'live':{'type':'boolean'},'categories':{'type':'array','items':{'type':'string'}}},'additionalProperties':False}
schemas['CredentialMedia']=media
page=envelope({'type':'object','properties':{'items':{'type':'array','items':{'$ref':'#/components/schemas/CredentialMedia'}},'nextAfter':nullable('integer')}})
for e in endpoints:
    route, method = e['path'],e['method'].lower()
    old=spec['paths'].get(route+'/{token}',{}).get(method)
    op=copy.deepcopy(old) if old else {'responses': {'200':response(page)}}
    op.update(tags=['Cred'],operationId='credential_'+e['kind'].replace('-','_'),summary=e['title'],description=e['description']+' Requires '+e['scope']+'. '+e['parameters']+' Returns: '+e['returns'],security=[{'ApiCredential':[]}])
    op['parameters']=[p for p in op.get('parameters',[]) if p['name']!='token']
    if '{uuid}' in route: op['parameters'].append(param('uuid','path'))
    if '{category}' in route: op['parameters'].append(param('category','path',{'type':'string','enum':['video','panorama','live','animated','screenshot','portrait']}))
    if e['kind'] in ('media','category-media'):
        op['parameters'] += [param('after',schema={'type':'integer','minimum':0}),param('limit',schema={'type':'integer','minimum':1,'maximum':200,'default':100})]
        if e['kind']=='media':op['parameters'].append(param('nsId',schema={'type':'integer','minimum':1}))
    if e['kind']=='detail':op['responses']['200']=response(envelope({'$ref':'#/components/schemas/CredentialMedia'}))
    if e['kind']=='categories':op['responses']['200']=response(envelope({'type':'object','properties':{'items':{'type':'array','items':{'type':'object','properties':{'category':{'type':'string'},'count':{'type':'integer'},'previews':{'type':'array','maxItems':3,'items':{'type':'object','properties':{k:v for k,v in media['properties'].items() if k in ('photoId','uuid','name','format','contentVersion')},'additionalProperties':False}}}}}}}))
    if e['kind'] in ('thumbnail','original'):
        op['responses']['200']={'description':'Binary media with its actual Content-Type','content':{'application/octet-stream':{'schema':{'type':'string','format':'binary'}}}}
        op['responses']['206']={'description':'Requested byte range'}
    for code,label in [('400','Invalid request'),('401','Missing, invalid or revoked API credential'),('403','Insufficient scope'),('404','Media/folder unavailable to this account'),('503','Storage or service unavailable')]:op['responses'][code]=response(envelope({'type':['string','null']}),label)
    spec['paths'].setdefault(route,{})[method]=op
    if old and e['kind']=='original':
        old['responses']['200']=copy.deepcopy(op['responses']['200'])
        old['responses']['206']={'description':'Requested byte range'}
        old['description']='Requires R. Returns original bytes with correct Content-Type; supports Range. Legacy path-token errors retain legacy responses.'
snapshot={'type':'object','properties':{'sampledAt':{'type':'string','format':'date-time'},'scope':{'type':'string'},'cpuPercent':{**nullable('number'),'minimum':0,'maximum':100},'backendBytes':nullable('integer'),'aiBytes':nullable('integer')}}
point={'type':'object','properties':{k:nullable('number') for k in ['cpuMean','cpuPeak','backendMeanBytes','aiMeanBytes','memoryPeakBytes']}}
point['properties'].update({k:{'type':'string','format':'date-time'} for k in ['bucket','firstSampleAt','lastSampleAt']})
point['properties'].update(bootId={'type':'string'},cpuSamples={'type':'integer'},memorySamples={'type':'integer'},partial={'type':'boolean'})
schemas.update(ResourceSnapshot=snapshot,ResourceHistoryPoint=point)
for route,data in [('/admin/resources/current',snapshot),('/admin/resources/history',{'type':'object','properties':{'items':{'type':'array','items':{'$ref':'#/components/schemas/ResourceHistoryPoint'}},'hours':{'type':'integer','enum':[12,24]},'bucketSeconds':{'type':'integer','const':600}}}),('/admin/runtime',{'type':'object','properties':{'bootId':{'type':'string'},'startedAt':{'type':'string','format':'date-time'},'uptimeSeconds':{'type':'integer','minimum':0}}})]:
    op={'tags':['Admin'],'summary':route.rsplit('/',1)[1]+' (admin only)','operationId':route.strip('/').replace('/','_'),'security':[{'SessionToken':[]}],'responses':{'200':response(envelope(data))}}
    if route.endswith('history'):op['parameters']=[param('hours',schema={'type':'integer','enum':[12,24],'default':24})]
    spec['paths'][route]={'get':op}
path.write_text(json.dumps(spec,separators=(',',':'),ensure_ascii=False)+'\n')
