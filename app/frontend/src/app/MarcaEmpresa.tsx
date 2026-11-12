import { useState } from 'react';
import huggingFace from '../arte/huggingface.svg';
import openRouter from '../../../../site/marcas/openrouter.svg';
import '../estilo/marca-empresa.css';

const catalogo: Record<string,string> = { 'hugging face':huggingFace, openrouter:openRouter };

// Os três destinos mostram a mesma marca. Nenhuma URL da vaga recebe uma visita só por abrir a lista.
export function MarcaEmpresa({nome, classe='rd-marca', tamanho}:{nome:string;classe?:string;tamanho?:number}) {
  const [quebrada, falhou]=useState('');
  const empresa=(nome || '').trim(), chave=empresa.toLocaleLowerCase('pt-BR');
  const arquivo=catalogo[chave];
  const letras=empresa.split(/\s+/).slice(0,2).map(p=>[...p][0]).join('').toLocaleUpperCase('pt-BR');
  const tom=[...chave].reduce((h,c)=>(h*31+c.charCodeAt(0))>>>0,0)%5;
  return <span className={classe+' empresa-marca '+(arquivo?'marca-local':'marca-tom-'+tom)}
    style={tamanho?{width:tamanho,height:tamanho,fontSize:tamanho*.38}:undefined} aria-hidden="true">
    {arquivo && quebrada!==arquivo ? <img src={arquivo} alt="" onError={()=>falhou(arquivo)}/> : letras}
  </span>;
}
