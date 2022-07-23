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
    Button, Text, useBoolean, InputRightElement, InputGroup
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
    const [oldPwd, setOldPwd] = useState<string>();
    const [newPwd, setNewPwd] = useState<string>();
    const token = useToken()
    const {post, isLoading***REMOVED*** = useApi()
    const [show, setShow] = useBoolean(false)

    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
***REMOVED***
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
***REMOVED***
    const handleEmailChange = (e: ChangeEvent<HTMLInputElement>) => {
        setEmail(e.target.value)
***REMOVED***
    const handleOldPwdChange = (e: ChangeEvent<HTMLInputElement>) => {
        setOldPwd(e.target.value)
***REMOVED***
    const handleNewPwdChange = (e: ChangeEvent<HTMLInputElement>) => {
        setNewPwd(e.target.value)
***REMOVED***
    const handleSubmit = async () => {
        let requested = false
        let success = true
        if (file !== undefined){
            requested = true
            await post("/user/updateAvatar", {
                file: file
    ***REMOVED***, {
                headers: {
                    "HRD-token": token,
                    "Content-type": "multipart/form-data"
   ***REMOVED*****REMOVED***
    ***REMOVED***).then(res=>{success &&= (!!res && res.data.msgCode === 200)***REMOVED***)
***REMOVED***
        if (username !== undefined) {
            requested = true
            await post(`/user/updateUsername?username=${username***REMOVED***`, {***REMOVED***, {
                headers: {
                    "HRD-token": token
   ***REMOVED*****REMOVED***
    ***REMOVED***).then(res=>{success &&= (!!res && res.data.msgCode === 200)***REMOVED***)
***REMOVED***
        if (email !== undefined) {
            requested = true
            await post(`/user/updateEmail?email=${email***REMOVED***`, {***REMOVED***, {
                headers: {
                    "HRD-token": token
   ***REMOVED*****REMOVED***
    ***REMOVED***).then(res=>{success &&= (!!res && res.data.msgCode === 200)***REMOVED***)
***REMOVED***
        if (oldPwd !== undefined && newPwd !== undefined) {
            requested = true
            await post(`/user/updatePassword?oldPass=${oldPwd***REMOVED***&newPass=${newPwd***REMOVED***`, {***REMOVED***, {
                headers:{
                    "HRD-token": token
   ***REMOVED*****REMOVED***
    ***REMOVED***).then(res=>{success &&= (!!res && res.data.msgCode === 200)***REMOVED***)
***REMOVED***
        if (success && requested) Router.reload()
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
                    <Input placeholder='Username' onChange={handleUsernameChange***REMOVED*** value={username***REMOVED***/>
                    <Input placeholder='Email' mt={4***REMOVED*** onChange={handleEmailChange***REMOVED*** value={email***REMOVED***/>
                    <Text fontWeight={"semibold"***REMOVED*** mt={4***REMOVED*** fontSize={"xl"***REMOVED***>Change Avatar</Text>
                    <Input type={"file"***REMOVED*** mt={4***REMOVED*** accept={"image/png, image/jpeg"***REMOVED*** onChange={handleFileChange***REMOVED***/>
                    <Text fontWeight={"semibold"***REMOVED*** mt={4***REMOVED*** fontSize={"xl"***REMOVED***>Change Password</Text>
                    <Input placeholder='Old Password' type={"password"***REMOVED*** mt={4***REMOVED*** onChange={handleOldPwdChange***REMOVED*** value={oldPwd***REMOVED***/>
                    <InputGroup mt={4***REMOVED***>
                        <Input variant="outline" type={show ? 'text' : 'password'***REMOVED*** placeholder="New password"
                               onChange={handleNewPwdChange***REMOVED*** isDisabled={isLoading***REMOVED***></Input>
                        <InputRightElement pr={1***REMOVED***>
                            <Button padding={4***REMOVED*** size='sm' onClick={setShow.toggle***REMOVED*** fontSize="xs">
             ***REMOVED*****REMOVED*****REMOVED*****REMOVED***show ? 'Hide' : 'Show'***REMOVED***
                            </Button>
                        </InputRightElement>
                    </InputGroup>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3***REMOVED*** onClick={onClose***REMOVED***>
                        Cancel
                    </Button>
                    <Button colorScheme='teal' onClick={() => {
                        handleSubmit();
                        onClose()
  ***REMOVED*****REMOVED*****REMOVED******REMOVED*** isLoading={isLoading***REMOVED*** loadingText={"Saving..."***REMOVED***>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
***REMOVED***

export default ChangeInfoDrawer