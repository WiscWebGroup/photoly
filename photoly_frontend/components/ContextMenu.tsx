import * as React from 'react';
import { MutableRefObject, useCallback, useEffect, useRef, useState ***REMOVED*** from 'react';
import { useEventListener, Portal, Menu, MenuButton, PortalProps, MenuButtonProps, MenuProps ***REMOVED*** from '@chakra-ui/react';

export interface ContextMenuProps<T extends HTMLElement> {
  renderMenu: () => JSX.Element | null;
  children: (ref: MutableRefObject<T | null>) => JSX.Element | null;
  menuProps?: MenuProps;
  portalProps?: PortalProps;
  menuButtonProps?: MenuButtonProps;
  stopPropagation?: boolean
***REMOVED***

export function ContextMenu<T extends HTMLElement = HTMLElement>(props: ContextMenuProps<T>) {
  const [isOpen, setIsOpen] = useState(false);
  const [isRendered, setIsRendered] = useState(false);
  const [isDeferredOpen, setIsDeferredOpen] = useState(false);
  const [position, setPosition] = useState([0, 0]);
  const targetRef = useRef<T>(null);

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => {
        setIsRendered(true);
        setTimeout(() => {
          setIsDeferredOpen(true);
***REMOVED***);
  ***REMOVED***);
***REMOVED*** else {
      setIsDeferredOpen(false);
      const timeout = setTimeout(() => {
        setIsRendered(isOpen);
  ***REMOVED***, 1000);
      return () => clearTimeout(timeout);
***REMOVED***
  ***REMOVED***, [isOpen]);

  const handleContextMenu = (e: any) => {
    if (targetRef.current?.contains(e.target as any) || e.target === targetRef.current) {
      e.preventDefault();
      if (props.stopPropagation) {
        e.stopPropagation()
        e.stopImmediatePropagation()
        console.log('functionig')
  ***REMOVED***

      setIsOpen(true);
      setPosition([e.pageX, e.pageY]);
***REMOVED*** else {
      setIsOpen(false);
***REMOVED***
  ***REMOVED***

  

  useEffect(() => {
    window.addEventListener('contextmenu', handleContextMenu)
    return () => {
      window.removeEventListener('contextmenu', handleContextMenu)
***REMOVED***
  ***REMOVED***)

  const onCloseHandler = useCallback(() => {
    props.menuProps?.onClose?.();
    setIsOpen(false);
  ***REMOVED***, [props.menuProps?.onClose, setIsOpen]);

  const handleClick = (event:any) => {
    console.log(event)
    if (event.type === "click" || event.type === "contextmenu") {
      event.preventDefault();
      setIsDeferredOpen(false)
***REMOVED***else {
        event.preventDefault();
***REMOVED***
  ***REMOVED***

  return (
    <>
      
  ***REMOVED***props.children(targetRef)***REMOVED***
  ***REMOVED***isRendered && (
        <Portal {...props.portalProps***REMOVED***>
          <Menu isOpen={isDeferredOpen***REMOVED*** gutter={0***REMOVED*** {...props.menuProps***REMOVED*** onClose={onCloseHandler***REMOVED***>
            <MenuButton
              aria-hidden={true***REMOVED***
              w={1***REMOVED***
              h={1***REMOVED***
              style={{
                position: 'absolute',
                left: position[0],
                top: position[1],
                cursor: 'default',
 ***REMOVED*****REMOVED******REMOVED***
***REMOVED*****REMOVED*****REMOVED***...props.menuButtonProps***REMOVED***
            />
   ***REMOVED*****REMOVED***props.renderMenu()***REMOVED***
          </Menu>
        </Portal>
      )***REMOVED***
  ***REMOVED***isDeferredOpen ? (<div style={{ "position": "fixed", "inset": "0px", "backgroundColor": "transparent" ***REMOVED******REMOVED*** onContextMenu={handleClick***REMOVED***>
        
        </div>) : ("")***REMOVED***
    </>
  );
***REMOVED***