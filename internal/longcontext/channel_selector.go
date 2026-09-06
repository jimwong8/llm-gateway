package longcontext

import (
	"sort"
	"sync"
	"time"
)

type ChannelHealth struct { ID string; Failures int; CooldownUntil time.Time; LastUsed time.Time }
type ChannelSelector struct { mu sync.Mutex; items map[string]*ChannelHealth; cooldown time.Duration }
func NewChannelSelector(ids []string, cooldown time.Duration)*ChannelSelector{if cooldown<=0{cooldown=30*time.Second};s:=&ChannelSelector{items:map[string]*ChannelHealth{},cooldown:cooldown};for _,id:=range ids{s.items[id]=&ChannelHealth{ID:id}};return s}
func(s *ChannelSelector) Pick(now time.Time)string{s.mu.Lock();defer s.mu.Unlock();a:=[]*ChannelHealth{};for _,x:=range s.items{if now.Before(x.CooldownUntil){continue};a=append(a,x)};sort.Slice(a,func(i,j int)bool{if a[i].Failures!=a[j].Failures{return a[i].Failures<a[j].Failures};return a[i].LastUsed.Before(a[j].LastUsed)});if len(a)==0{return ""};a[0].LastUsed=now;return a[0].ID}
func(s *ChannelSelector) Success(id string){s.mu.Lock();defer s.mu.Unlock();if x:=s.items[id];x!=nil{x.Failures=0;x.CooldownUntil=time.Time{}}}
func(s *ChannelSelector) Failure(id string, status int, now time.Time){s.mu.Lock();defer s.mu.Unlock();if x:=s.items[id];x==nil{return}else if status==403{x.Failures=99;x.CooldownUntil=now.Add(365*24*time.Hour)}else if status==429{x.CooldownUntil=now.Add(s.cooldown)}else{x.Failures++;x.CooldownUntil=now.Add(time.Duration(x.Failures)*time.Second)}}
