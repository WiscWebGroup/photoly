import {Avatar, Box, Button, Center, Container, HStack, Image, VStack, Text, Link***REMOVED*** from "@chakra-ui/react";
import router from "next/router";
import React from "react";
import { ExternalLinkIcon ***REMOVED*** from '@chakra-ui/icons'

const Home:React.FC = () => {
  return (
      <>
          <Center h={"100vh"***REMOVED*** bgImage={"/JH_20211030_5406.jpg"***REMOVED*** bgSize={"cover"***REMOVED*** position={"relative"***REMOVED*** bgAttachment={"fixed"***REMOVED*** bgPos={"center 0"***REMOVED*** bgRepeat={"no-repeat"***REMOVED***>
          <HStack bg={"blackAlpha.300"***REMOVED*** w={"100%"***REMOVED*** h={16***REMOVED*** color={"white"***REMOVED*** p={4***REMOVED*** position={"fixed"***REMOVED*** top={0***REMOVED***
                justifyContent={"space-between"***REMOVED***>
            <HStack  cursor={"default"***REMOVED***>
                <Image src={"/favicon.ico"***REMOVED*** alt={"logo"***REMOVED*** width={"50px"***REMOVED*** height={"50px"***REMOVED*** data-testid={"logo"***REMOVED***/>
                <Text fontSize={"xl"***REMOVED*** fontWeight={"medium"***REMOVED***>PHOTOLY</Text>
            </HStack>
            <HStack h={"inherit"***REMOVED*** p={2***REMOVED*** spacing={4***REMOVED*** mr={4***REMOVED***>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED*** onClick={() => router.push("/login")***REMOVED***>Sign In</Button>
                <Button variant={"link"***REMOVED*** color={"white"***REMOVED*** onClick={() => router.push("/signup")***REMOVED***>Sign Up</Button>
            </HStack>
        </HStack>
                <VStack >
                <Text fontSize='6xl' color={"whiteAlpha.900"***REMOVED***>See the Value of Photo Inside</Text>
                <Text fontSize='6xl' style={{marginTop:"-1rem"***REMOVED******REMOVED*** color={"orange.50"***REMOVED***>The Beauty Shines Here</Text>
                <Text fontSize='2xl' color={"teal.100"***REMOVED***>Photoly - Share Your Inspiration</Text>
                <HStack style={{marginTop:"3rem"***REMOVED******REMOVED***>
                <Link href="/signup">
                <Button colorScheme='whiteAlpha'>
                    Sign Up
                </Button>
                </Link>
                <Link href="/login">
                <Button colorScheme='gray'>
                    Sign In
                </Button>
                </Link>
                
                </HStack>
                </VStack>
              
          </Center>

          <Center h={"100vh"***REMOVED*** bgImage={"/JH_20220710_5856.jpg"***REMOVED*** bgSize={"cover"***REMOVED*** position={"relative"***REMOVED*** bgAttachment={"fixed"***REMOVED*** bgPos={"center 0"***REMOVED*** bgRepeat={"no-repeat"***REMOVED***>
          <VStack color={"teal.900"***REMOVED*** spacing={"2rem"***REMOVED***>
            <HStack>
            <Text fontSize='6xl' color={"whiteAlpha.900"***REMOVED*** fontWeight={"bold"***REMOVED*** fontFamily={"helvetica"***REMOVED***>Project项目</Text>
            <Image src={"/favicon.ico"***REMOVED*** alt={"logo"***REMOVED*** width={"9rem"***REMOVED*** height={"9rem"***REMOVED*** data-testid={"logo"***REMOVED***/>
            <Text fontSize='6xl' color={"whiteAlpha.900"***REMOVED*** fontWeight={"bold"***REMOVED*** fontFamily={"helvetica"***REMOVED***>Information</Text>
            </HStack>
          
          <Container maxW='xl' bg='whiteAlpha.700' rounded={"2xl"***REMOVED*** centerContent>
                <Box padding='4' bg='whiteAlpha.200' maxW='md'>
                    There are demanding for self-host and reliable picture host application. Photoly provides a solution just right for that. Using Photoly, you can manage
                    your pictures and photos with easy clicks, categorize them using tags, galleries, and namespaces. Moreover, Photoly provides API for you to access our 
                    functionalities for embedding in other applications.
                </Box>
            </Container>

            <Container maxW='xl' bg='whiteAlpha.700' rounded={"2xl"***REMOVED*** centerContent>
                <Box padding='4' bg='whiteAlpha.200'maxW='md'>
                <Link href='https://github.com/WiscWebGroup/photoly' alignSelf={"flex-start"***REMOVED*** isExternal>
                            Star our GitHub Repo! <ExternalLinkIcon mx='2px' />
                            </Link>
                </Box>
            </Container>
          </VStack>
          </Center>

          <Center h={"100vh"***REMOVED*** bgImage={"/JH_20220601_5686.jpg"***REMOVED*** bgSize={"cover"***REMOVED*** position={"relative"***REMOVED*** bgAttachment={"fixed"***REMOVED*** bgPos={"center 0"***REMOVED*** bgRepeat={"no-repeat"***REMOVED***>
          <VStack >
                <Text fontSize='6xl' color={"blackAlpha.700"***REMOVED*** fontWeight={"bold"***REMOVED*** fontFamily={"helvetica"***REMOVED***>Us. About.</Text>
                <Container maxW='2xl' centerContent >
                    <VStack spacing={"20px"***REMOVED*** m={"20px"***REMOVED***>
                    <HStack bg='whiteAlpha.600' rounded={"2xl"***REMOVED***>
                        <Image src="https://avatars.githubusercontent.com/u/108884969" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/WiscWebGroup' alignSelf={"flex-start"***REMOVED***>
                            <Text fontSize='2xl'>WiscWebGroup<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>We are WiscWebGroup, a team located in Madison, Wisconsin. We mainly develop interesting web applications.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>
                    <HStack bg='whiteAlpha.600' rounded={"2xl"***REMOVED***>
                        <Image src="https://avatars.githubusercontent.com/u/68500948" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/Harold-y' alignSelf={"flex-start"***REMOVED***>
                            <Text fontSize='2xl'>Yixuan Ye (Harold-y)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Student at UW-Madison major in CS & DS. I did the database, backend, continuous integration, and part of the frontend work of this project. I also designed the logo.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>

                    <HStack bg='whiteAlpha.600' rounded={"2xl"***REMOVED***>
                        <Image src="https://avatars.githubusercontent.com/u/31459252" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/Slijeff' alignSelf={"flex-start"***REMOVED***>
                            <Text fontSize='2xl'>Jeffrey Hui (Slijeff)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Student at UW-Madison major in CS & DS. I did part of the frontend job in this project. I photographed the background images for the home page.</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>

                    <HStack bg='whiteAlpha.600' rounded={"2xl"***REMOVED***>
                        <Image src="https://avatars.githubusercontent.com/u/46753484" 
                        borderRadius='full'
                         boxSize='150px'
                         padding='4'>
                        </Image>

                        <Box padding='4' color='black' maxW='md'>
                            <VStack>
                            
                            <Link href='https://github.com/NataRich' alignSelf={"flex-start"***REMOVED***>
                            <Text fontSize='2xl'>Zihan Zhao (NataRich)<ExternalLinkIcon mx='2px' /></Text>
                            </Link>
                                <Text>Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam</Text>
                               
                            </VStack>
                        </Box>
                    </HStack>
                    </VStack>
                    
                
            </Container>
            </VStack>
          
          </Center>
      </>


  )
***REMOVED***

export default Home
