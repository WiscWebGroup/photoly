import React, {ChangeEvent, useState***REMOVED*** from "react";
import {
    Drawer,
    DrawerBody,
    DrawerFooter,
    DrawerHeader,
    DrawerOverlay,
    DrawerContent,
    DrawerCloseButton,
    Input,
    Button
***REMOVED*** from '@chakra-ui/react'
import useApi from "../hooks/useApi";
import useToken from "../hooks/useToken";
import Router from "next/router"

interface NameAndAvatarDrawerProps {
    isOpen: boolean,
    onClose: () => void
***REMOVED***

const NameAndAvatarDrawer: React.FC<NameAndAvatarDrawerProps> = ({isOpen, onClose***REMOVED***) => {
    const [file, setFile] = useState<File>()
    const [username, setUsername] = useState<string>()
    const token = useToken()
    const {post***REMOVED*** = useApi()
    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
***REMOVED***
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
***REMOVED***
    const handleSubmit = async () => {
        if (!!file){
            await post("/user/updateAvatar", {
                file: file
    ***REMOVED***, {
                headers: {
                    "HRD-token": token,
                    "Content-type": "multipart/form-data"
   ***REMOVED*****REMOVED***
    ***REMOVED***)
***REMOVED***
        if (!!username) {
            await post(`/user/updateUsername?username=${username***REMOVED***`, {***REMOVED***, {
                headers: {
                    "HRD-token": token
   ***REMOVED*****REMOVED***
    ***REMOVED***)
***REMOVED***
        Router.reload()
***REMOVED***

    return (
        <Drawer
            isOpen={isOpen***REMOVED***
            placement='right'
            onClose={onClose***REMOVED***
        >
            <DrawerOverlay/>
            <DrawerContent>
                <DrawerCloseButton/>
                <DrawerHeader>Change Username & Avatar</DrawerHeader>
                <DrawerBody>
                    <Input placeholder='Username' onChange={handleUsernameChange***REMOVED***/>
                    <Input type={"file"***REMOVED*** mt={4***REMOVED*** accept={"image/png, image/jpeg"***REMOVED*** onChange={handleFileChange***REMOVED***/>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3***REMOVED*** onClick={onClose***REMOVED***>
                        Cancel
                    </Button>
                    <Button colorScheme='blue' onClick={() => {
                        handleSubmit();
                        onClose()
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
***REMOVED***

export default NameAndAvatarDrawer