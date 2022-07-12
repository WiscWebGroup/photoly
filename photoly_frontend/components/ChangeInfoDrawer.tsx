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

interface ChangeInfoProps {
    isOpen: boolean,
    onClose: () => void
***REMOVED***

const ChangeInfoDrawer: React.FC<ChangeInfoProps> = ({isOpen, onClose***REMOVED***) => {
    const [file, setFile] = useState<File>()
    const [username, setUsername] = useState<string>()
    const [email, setEmail] = useState<string>()
    const token = useToken()
    const {post, isLoading***REMOVED*** = useApi()
    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
***REMOVED***
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
***REMOVED***
    const handleEmailChange = (e: ChangeEvent<HTMLInputElement>) => {
        setEmail(e.target.value)
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
        if (!!email) {
            await post(`/user/updateEmail?email=${email***REMOVED***`, {***REMOVED***, {
                headers: {
                    "HRD-token": token
   ***REMOVED*****REMOVED***
    ***REMOVED***)
***REMOVED***
        if (!!email || !!username || !!file) Router.reload()
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
                <DrawerHeader>Change Info</DrawerHeader>
                <DrawerBody>
                    <Input placeholder='Username' onChange={handleUsernameChange***REMOVED***/>
                    <Input placeholder='Email' mt={4***REMOVED*** onChange={handleEmailChange***REMOVED***/>
                    <Input type={"file"***REMOVED*** mt={4***REMOVED*** accept={"image/png, image/jpeg"***REMOVED*** onChange={handleFileChange***REMOVED***/>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3***REMOVED*** onClick={onClose***REMOVED***>
                        Cancel
                    </Button>
                    <Button colorScheme='blue' onClick={() => {
                        handleSubmit();
                        onClose()
  ***REMOVED*****REMOVED*****REMOVED******REMOVED*** isLoading={isLoading***REMOVED*** loadingText={"Saving..."***REMOVED***>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
***REMOVED***

export default ChangeInfoDrawer